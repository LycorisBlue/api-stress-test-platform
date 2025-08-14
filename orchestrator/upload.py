import json
import csv
import io
import os
import re
from typing import Optional, List, Dict, Any
from fastapi import APIRouter, File, UploadFile, Depends
from pydantic import BaseModel
from auth import get_current_user

# Models
class ValidationResult(BaseModel):
   status: str  # "success" ou "error"
   message: str
   errors: List[str] = []
   warnings: List[str] = []
   analysis: Dict[str, Any] = {}

# Configuration
UPLOAD_DIR = "/tmp/loadtest"
os.makedirs(UPLOAD_DIR, exist_ok=True)

# Fonctions utilitaires
def extract_variables_from_text(text: str) -> List[str]:
   """Extrait toutes les variables {{xxx}} d'un texte"""
   pattern = r'\{\{([^}]+)\}\}'
   return re.findall(pattern, text)

def extract_variables_from_scenario(scenario_data: dict) -> Dict[str, List[str]]:
   """Extrait toutes les variables du scénario et les catégorise"""
   variables = {
       "user": [],  # Variables {{user.xxx}}
       "env": [],   # Variables {{env.xxx}} ou autres
       "extract": [] # Variables extraites d'étapes précédentes
   }
   
   scenario_text = json.dumps(scenario_data)
   all_vars = extract_variables_from_text(scenario_text)
   
   for var in all_vars:
       if var.startswith("user."):
           column_name = var.replace("user.", "")
           if column_name not in variables["user"]:
               variables["user"].append(column_name)
       elif var.startswith("env."):
           env_var = var.replace("env.", "")
           if env_var not in variables["env"]:
               variables["env"].append(env_var)
       else:
           # Variables extraites ou autres
           if var not in variables["extract"]:
               variables["extract"].append(var)
   
   return variables

def validate_scenario_structure(scenario_data: dict) -> List[str]:
   """Valide la structure basique du scénario"""
   errors = []
   
   if not isinstance(scenario_data, dict):
       errors.append("Le scénario doit être un objet JSON")
       return errors
   
   if "name" not in scenario_data:
       errors.append("Le scénario doit avoir un 'name'")
   
   if "steps" not in scenario_data:
       errors.append("Le scénario doit avoir des 'steps'")
       return errors
   
   if not isinstance(scenario_data["steps"], list):
       errors.append("Les 'steps' doivent être une liste")
       return errors
   
   if len(scenario_data["steps"]) == 0:
       errors.append("Le scénario doit avoir au moins une étape")
   
   # Validation des étapes
   for i, step in enumerate(scenario_data["steps"]):
       if not isinstance(step, dict):
           errors.append(f"L'étape {i+1} doit être un objet")
           continue
       
       required_fields = ["name", "method", "url"]
       for field in required_fields:
           if field not in step:
               errors.append(f"L'étape {i+1} '{step.get('name', 'sans nom')}' manque le champ '{field}'")
       
       if "method" in step and step["method"] not in ["GET", "POST", "PUT", "DELETE", "PATCH"]:
           errors.append(f"L'étape {i+1} a une méthode HTTP invalide: {step['method']}")
   
   return errors

def validate_variables_structure(variables_data: dict) -> List[str]:
   """Valide la structure du fichier variables"""
   errors = []
   
   if not isinstance(variables_data, dict):
       errors.append("Le fichier variables doit être un objet JSON")
       return errors
   
   # Validation du mode
   if "mode" not in variables_data:
       errors.append("Le fichier variables doit spécifier un 'mode'")
   elif variables_data["mode"] not in ["users", "requests"]:
       errors.append("Le mode doit être 'users' ou 'requests'")
   
   # Validation selon le mode
   mode = variables_data.get("mode")
   if mode == "users":
       if "virtualUsers" not in variables_data:
           errors.append("Le mode 'users' nécessite le champ 'virtualUsers'")
       elif not isinstance(variables_data["virtualUsers"], int) or variables_data["virtualUsers"] <= 0:
           errors.append("'virtualUsers' doit être un entier positif")
   
   if mode == "requests":
       if "totalRequests" not in variables_data:
           errors.append("Le mode 'requests' nécessite le champ 'totalRequests'")
       elif not isinstance(variables_data["totalRequests"], int) or variables_data["totalRequests"] <= 0:
           errors.append("'totalRequests' doit être un entier positif")
   
   return errors

def validate_csv_structure(csv_content: str) -> tuple[List[str], List[str]]:
   """Valide la structure du CSV et retourne (erreurs, colonnes)"""
   errors = []
   columns = []
   
   try:
       csv_reader = csv.reader(io.StringIO(csv_content))
       headers = next(csv_reader, None)
       
       if not headers:
           errors.append("Le fichier CSV est vide ou n'a pas d'en-têtes")
           return errors, columns
       
       # Nettoyer les colonnes (supprimer espaces)
       columns = [col.strip() for col in headers if col.strip()]
       
       if len(columns) == 0:
           errors.append("Le fichier CSV n'a pas de colonnes valides")
           return errors, columns
       
       # Vérifier qu'il y a au moins une ligne de données
       row_count = 0
       for row in csv_reader:
           if any(cell.strip() for cell in row):  # Au moins une cellule non vide
               row_count += 1
       
       if row_count == 0:
           errors.append("Le fichier CSV n'a pas de données (seulement des en-têtes)")
       
   except Exception as e:
       errors.append(f"Erreur de parsing CSV: {str(e)}")
   
   return errors, columns

def validate_cross_file_consistency(scenario_vars: Dict[str, List[str]], 
                                 variables_data: dict, 
                                 csv_columns: List[str]) -> List[str]:
   """Valide la cohérence entre les 3 fichiers"""
   errors = []
   
   # 1. Vérifier que les variables user.xxx existent dans le CSV
   for user_var in scenario_vars["user"]:
       if user_var not in csv_columns:
           errors.append(f"Variable '{{{{user.{user_var}}}}}' utilisée dans le scénario mais colonne '{user_var}' manquante dans users.csv")
   
   # 2. Vérifier que les variables env.xxx existent dans variables.json
   env_section = variables_data.get("environment", {})
   for env_var in scenario_vars["env"]:
       if env_var not in env_section:
           errors.append(f"Variable '{{{{env.{env_var}}}}}' utilisée dans le scénario mais '{env_var}' manquant dans variables.json > environment")
   
   # 3. Vérifier cohérence mode vs présence du CSV
   mode = variables_data.get("mode")
   if mode == "users" and len(csv_columns) == 0:
       errors.append("Mode 'users' spécifié mais fichier users.csv manquant ou vide")
   
   return errors

# Router
router = APIRouter(prefix="/upload", tags=["upload"])

@router.post("/validate", response_model=ValidationResult)
async def validate_files(
   current_user: str = Depends(get_current_user),
   scenario: UploadFile = File(...),
   variables: UploadFile = File(...),
   users: Optional[UploadFile] = File(None)
):
   """Upload et validation intelligente des 3 fichiers avec analyse de cohérence"""
   
   errors = []
   warnings = []
   analysis = {}
   
   # 1. Lecture et parsing des fichiers
   try:
       # Lire scenario.json
       scenario_content = await scenario.read()
       scenario_data = json.loads(scenario_content.decode('utf-8'))
       
       # Lire variables.json
       variables_content = await variables.read()
       variables_data = json.loads(variables_content.decode('utf-8'))
       
       # Lire users.csv (optionnel)
       csv_columns = []
       if users:
           users_content = await users.read()
           csv_content = users_content.decode('utf-8')
           csv_errors, csv_columns = validate_csv_structure(csv_content)
           errors.extend(csv_errors)
       
   except json.JSONDecodeError as e:
       errors.append(f"Erreur JSON: {str(e)}")
       return ValidationResult(
           status="error",
           message="Erreur de parsing des fichiers JSON",
           errors=errors
       )
   except Exception as e:
       errors.append(f"Erreur de lecture: {str(e)}")
       return ValidationResult(
           status="error", 
           message="Erreur de lecture des fichiers",
           errors=errors
       )
   
   # 2. Validation structure de chaque fichier
   scenario_errors = validate_scenario_structure(scenario_data)
   errors.extend(scenario_errors)
   
   variables_errors = validate_variables_structure(variables_data)
   errors.extend(variables_errors)
   
   # 3. Extraction des variables du scénario
   scenario_vars = extract_variables_from_scenario(scenario_data)
   analysis["variables_found"] = scenario_vars
   analysis["csv_columns"] = csv_columns
   
   # 4. Validation de cohérence cross-fichiers
   if not scenario_errors and not variables_errors:  # Seulement si pas d'erreurs structurelles
       consistency_errors = validate_cross_file_consistency(scenario_vars, variables_data, csv_columns)
       errors.extend(consistency_errors)
   
   # 5. Warnings et informations
   if users is None and scenario_vars["user"]:
       warnings.append(f"Fichier users.csv non fourni mais variables utilisateur détectées: {scenario_vars['user']}")
   
   if not scenario_vars["user"] and users:
       warnings.append("Fichier users.csv fourni mais aucune variable utilisateur détectée dans le scénario")
   
   # 6. Sauvegarde si tout est valide
   if not errors:
       try:
           # Sauvegarder les fichiers validés
           with open(f"{UPLOAD_DIR}/scenario.json", "w") as f:
               json.dump(scenario_data, f, indent=2)
           
           with open(f"{UPLOAD_DIR}/variables.json", "w") as f:
               json.dump(variables_data, f, indent=2)
           
           if users:
               with open(f"{UPLOAD_DIR}/users.csv", "wb") as f:
                   f.write(users_content)
           
           analysis["files_saved"] = True
           
       except Exception as e:
           errors.append(f"Erreur de sauvegarde: {str(e)}")
   
   # 7. Résultat final
   if errors:
       return ValidationResult(
           status="error",
           message=f"Validation échouée: {len(errors)} erreur(s) détectée(s)",
           errors=errors,
           warnings=warnings,
           analysis=analysis
       )
   else:
       return ValidationResult(
           status="success",
           message="Tous les fichiers sont valides et cohérents",
           warnings=warnings,
           analysis=analysis
       )
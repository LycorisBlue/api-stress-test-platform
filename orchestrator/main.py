import json
import csv
import io
import os
import re
import requests
import uuid
from typing import Optional, List, Dict, Any
from datetime import datetime
from fastapi import FastAPI, File, UploadFile, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel

app = FastAPI(title="Orchestrator API", version="0.1.0")

app.add_middleware(
   CORSMiddleware,
   allow_origins=["*"], 
   allow_credentials=True,
   allow_methods=["*"], 
   allow_headers=["*"],
)

# Stockage temporaire
UPLOAD_DIR = "/tmp/loadtest"
os.makedirs(UPLOAD_DIR, exist_ok=True)
WORKER_URL = os.getenv("WORKER_URL", "http://worker:8090")

class ValidationResult(BaseModel):
   status: str  # "success" ou "error"
   message: str
   errors: List[str] = []
   warnings: List[str] = []
   analysis: Dict[str, Any] = {}

class ExecutionResponse(BaseModel):
   status: str  # "success", "failed", "timeout"
   test_id: str
   message: str
   summary: Optional[Dict[str, Any]] = None
   report_path: Optional[str] = None
   error: Optional[str] = None
   duration: Optional[str] = None

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

def parse_csv_to_dict(csv_content: str) -> List[Dict[str, str]]:
   """Convertit le contenu CSV en liste de dictionnaires"""
   try:
       csv_reader = csv.DictReader(io.StringIO(csv_content))
       users = []
       
       for row in csv_reader:
           # Nettoyer les espaces dans les clés et valeurs
           clean_row = {k.strip(): v.strip() for k, v in row.items()}
           users.append(clean_row)
       
       return users
   
   except Exception as e:
       print(f"Erreur de parsing CSV: {e}")
       return []

@app.get("/health")
def health():
   return {"status": "ok", "component": "orchestrator"}

@app.post("/upload/validate", response_model=ValidationResult)
async def validate_files(
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

@app.post("/execute", response_model=ExecutionResponse)
async def execute_test():
   """Lance l'exécution d'un test de charge via le worker Go"""
   
   # Vérifier que les fichiers validés existent
   scenario_path = f"{UPLOAD_DIR}/scenario.json"
   variables_path = f"{UPLOAD_DIR}/variables.json"
   users_path = f"{UPLOAD_DIR}/users.csv"
   
   if not os.path.exists(scenario_path) or not os.path.exists(variables_path):
       return ExecutionResponse(
           status="failed",
           test_id="",
           message="Fichiers de configuration manquants",
           error="Veuillez d'abord uploader et valider scenario.json et variables.json"
       )
   
   try:
       # Lire les fichiers de configuration
       with open(scenario_path, 'r') as f:
           scenario_data = json.load(f)
       
       with open(variables_path, 'r') as f:
           variables_data = json.load(f)
       
       # Lire le fichier users.csv si présent
       users_data = []
       if os.path.exists(users_path):
           with open(users_path, 'r') as f:
               csv_content = f.read()
               users_data = parse_csv_to_dict(csv_content)
       
       # Générer un ID unique pour ce test
       test_id = f"test_{int(datetime.now().timestamp())}"
       
       # Préparer la configuration pour le worker
       test_config = {
           "mode": variables_data.get("mode", "users"),
           "virtualUsers": variables_data.get("virtualUsers", 1),
           "totalRequests": variables_data.get("totalRequests", 100),
           "duration": variables_data.get("duration", "2m"),
           "warmup": variables_data.get("warmup", "30s"),
           "environment": variables_data.get("environment", {}),
           "scenario": scenario_data,
           "usersData": users_data
       }
       
       # Préparer la requête pour le worker
       worker_request = {
           "test_id": test_id,
           "config": test_config,
           "timestamp": datetime.now().isoformat() + "Z"
       }
       
       # Envoyer la requête au worker Go
       worker_response = requests.post(
           f"{WORKER_URL}/execute",
           json=worker_request,
           timeout=300  # 5 minutes de timeout
       )
       
       if worker_response.status_code == 200:
           result = worker_response.json()
           
           return ExecutionResponse(
               status=result.get("status", "completed"),
               test_id=test_id,
               message=result.get("message", "Test exécuté avec succès"),
               summary=result.get("summary"),
               report_path=result.get("report_path"),
               error=result.get("error"),
               duration=result.get("summary", {}).get("duration") if result.get("summary") else None
           )
       else:
           return ExecutionResponse(
               status="failed",
               test_id=test_id,
               message="Erreur de communication avec le worker",
               error=f"HTTP {worker_response.status_code}: {worker_response.text}"
           )
   
   except requests.exceptions.Timeout:
       return ExecutionResponse(
           status="timeout",
           test_id=test_id,
           message="Timeout lors de l'exécution du test",
           error="Le test a pris plus de 5 minutes à s'exécuter"
       )
   
   except requests.exceptions.ConnectionError:
       return ExecutionResponse(
           status="failed",
           test_id=test_id,
           message="Impossible de contacter le worker",
           error="Vérifiez que le service worker est démarré"
       )
   
   except Exception as e:
       return ExecutionResponse(
           status="failed",
           test_id=test_id,
           message="Erreur interne lors de l'exécution",
           error=str(e)
       )

@app.get("/worker/health")
async def check_worker_health():
   """Vérifie que le worker est accessible"""
   try:
       response = requests.get(f"{WORKER_URL}/health", timeout=5)
       if response.status_code == 200:
           worker_status = response.json()
           return {
               "status": "connected",
               "worker_status": worker_status,
               "worker_url": WORKER_URL
           }
       else:
           return {
               "status": "error",
               "message": f"Worker responded with status {response.status_code}",
               "worker_url": WORKER_URL
           }
   except Exception as e:
       return {
           "status": "unreachable",
           "error": str(e),
           "worker_url": WORKER_URL
       }
import json
import os
import requests
from datetime import datetime
from typing import Optional, Dict, Any
from fastapi import APIRouter, HTTPException, Depends
from pydantic import BaseModel
from auth import get_current_user

# Models
class ExecutionResponse(BaseModel):
   status: str  # "success", "failed", "timeout"
   test_id: str
   message: str
   summary: Optional[Dict[str, Any]] = None
   report_path: Optional[str] = None
   error: Optional[str] = None
   duration: Optional[str] = None

# Configuration
UPLOAD_DIR = "/tmp/loadtest"
WORKER_URL = os.getenv("WORKER_URL", "http://worker:8090")

# Fonctions utilitaires
def parse_csv_to_dict(csv_content: str) -> list[Dict[str, str]]:
   """Convertit le contenu CSV en liste de dictionnaires"""
   import csv
   import io
   
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

# Router
router = APIRouter(prefix="/execute", tags=["execution"])

@router.post("", response_model=ExecutionResponse)
async def execute_test(current_user: str = Depends(get_current_user)):
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

@router.get("/worker/health")
async def check_worker_health(current_user: str = Depends(get_current_user)):
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
import json
import os
from datetime import datetime, timedelta
from typing import Optional, Dict, Any
from fastapi import HTTPException, Depends, status
from fastapi.security import HTTPBearer, HTTPAuthorizationCredentials
from pydantic import BaseModel
from jose import jwt

# Configuration JWT
SECRET_KEY = os.getenv("JWT_SECRET_KEY", "your-secret-key-change-this-in-production")
ALGORITHM = "HS256"
ACCESS_TOKEN_EXPIRE_MINUTES = 480  # 8 heures

security = HTTPBearer()

# Models Pydantic
class LoginRequest(BaseModel):
   username: str
   password: str

class TokenResponse(BaseModel):
   access_token: str
   token_type: str
   expires_in: int
   username: str

# Chargement des utilisateurs admin
def load_admin_users() -> Dict[str, str]:
   """Charge les utilisateurs admin depuis admin_users.json"""
   try:
       with open("admin_users.json", "r") as f:
           return json.load(f)
   except FileNotFoundError:
       # Créer un fichier par défaut si inexistant
       default_users = {
           "admin": "admin123"
       }
       with open("admin_users.json", "w") as f:
           json.dump(default_users, f, indent=2)
       return default_users

# Services d'authentification
def authenticate_user(username: str, password: str) -> bool:
   """Authentifie un utilisateur"""
   users = load_admin_users()
   return users.get(username) == password

def create_access_token(username: str) -> str:
   """Crée un token JWT"""
   expires_delta = timedelta(minutes=ACCESS_TOKEN_EXPIRE_MINUTES)
   expire = datetime.utcnow() + expires_delta
   
   to_encode = {
       "sub": username,
       "exp": expire
   }
   
   return jwt.encode(to_encode, SECRET_KEY, algorithm=ALGORITHM)

def verify_token(token: str) -> Optional[str]:
   """Vérifie et décode un token JWT, retourne le username"""
   try:
       payload = jwt.decode(token, SECRET_KEY, algorithms=[ALGORITHM])
       username = payload.get("sub")
       return username
   except jwt.PyJWTError:
       return None

# Middleware de protection
async def get_current_user(credentials: HTTPAuthorizationCredentials = Depends(security)) -> str:
   """Middleware pour récupérer l'utilisateur actuel"""
   credentials_exception = HTTPException(
       status_code=status.HTTP_401_UNAUTHORIZED,
       detail="Invalid authentication credentials",
       headers={"WWW-Authenticate": "Bearer"},
   )
   
   username = verify_token(credentials.credentials)
   if username is None:
       raise credentials_exception
   
   return username

# Routes d'authentification
from fastapi import APIRouter

router = APIRouter(prefix="/auth", tags=["auth"])

@router.post("/login", response_model=TokenResponse)
async def login(login_data: LoginRequest):
   """Endpoint de connexion"""
   if not authenticate_user(login_data.username, login_data.password):
       raise HTTPException(
           status_code=status.HTTP_401_UNAUTHORIZED,
           detail="Incorrect username or password"
       )
   
   access_token = create_access_token(login_data.username)
   
   return TokenResponse(
       access_token=access_token,
       token_type="bearer",
       expires_in=ACCESS_TOKEN_EXPIRE_MINUTES * 60,
       username=login_data.username
   )

@router.get("/verify")
async def verify_current_user(current_user: str = Depends(get_current_user)):
   """Endpoint pour vérifier un token"""
   return {
       "status": "valid",
       "username": current_user
   }

@router.post("/logout")
async def logout():
   """Endpoint de déconnexion (côté client doit supprimer le token)"""
   return {"message": "Successfully logged out"}
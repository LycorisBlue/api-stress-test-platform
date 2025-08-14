import os
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

# Imports des routers
from auth import router as auth_router
from upload import router as upload_router
from execution import router as execution_router

app = FastAPI(title="Orchestrator API", version="0.1.0")

app.add_middleware(
   CORSMiddleware,
   allow_origins=["*"], 
   allow_credentials=True,
   allow_methods=["*"], 
   allow_headers=["*"],
)

# Montage des routers
app.include_router(auth_router)
app.include_router(upload_router)
app.include_router(execution_router)

# Endpoint de santé (non protégé)
@app.get("/health")
def health():
   return {"status": "ok", "component": "orchestrator"}
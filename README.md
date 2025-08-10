## 📝 **Mise à jour du README.md avec exemples de fichiers**

# API Load & Stress Test Tool

## 🚀 Description
Outil de test de charge et de stress pour APIs.
- Upload de fichiers `scenario.json`, `variables.json`, `users.csv`
- Modes : Virtual Users (VUs) ou nombre de requêtes
- Visualisation en temps réel des métriques
- Export JSON/CSV
- Observabilité via Prometheus et Grafana

## 📂 Structure du projet
```
ui/             # Interface Next.js (TypeScript + Tailwind)
orchestrator/   # API centrale (FastAPI)
workers/        # Workers Go pour exécution des tests
docker-compose.yml
```

## 🛠 Installation rapide
```bash
git clone https://github.com/LycorisBlue/api-stress-test-platform.git
cd <nom-du-repo>
docker compose up
```

## 📋 Modèles de fichiers de configuration

### 🎯 scenario.json - Scénario de test
Décrit les étapes du test à exécuter :
```json
{
  "name": "Test API E-commerce avec authentification",
  "steps": [
    {
      "name": "Login utilisateur",
      "method": "POST",
      "url": "{{env.baseUrl}}/api/auth/login",
      "headers": {
        "Content-Type": "application/json"
      },
      "body": {
        "email": "{{user.email}}",
        "password": "{{user.password}}"
      },
      "extract": {
        "authToken": "$.data.token"
      }
    },
    {
      "name": "Récupérer profil",
      "method": "GET",
      "url": "{{env.baseUrl}}/api/user/profile",
      "headers": {
        "Authorization": "Bearer {{authToken}}"
      }
    },
    {
      "name": "Ajouter au panier",
      "method": "POST",
      "url": "{{env.baseUrl}}/api/cart/add",
      "headers": {
        "Authorization": "Bearer {{authToken}}",
        "Content-Type": "application/json"
      },
      "body": {
        "productId": "{{user.favoriteProduct}}",
        "quantity": 1
      }
    }
  ]
}
```

### ⚙️ variables.json - Configuration du test
Définit comment exécuter le test :
```json
{
  "mode": "users",
  "virtualUsers": 10,
  "duration": "2m",
  "warmup": "30s",
  "environment": {
    "baseUrl": "https://api.monshop.com",
    "timeout": 5000
  },
  "thresholds": {
    "maxP95": 300,
    "maxErrorRate": 0.02
  }
}
```

**Modes disponibles :**
- `"users"` : Nombre d'utilisateurs virtuels (nécessite `virtualUsers`)
- `"requests"` : Nombre total de requêtes (nécessite `totalRequests`)

### 👥 users.csv - Données utilisateurs (optionnel)
Fournit les données pour chaque utilisateur virtuel :
```csv
email,password,favoriteProduct
john.doe@test.com,password123,PROD001
jane.smith@test.com,securePass456,PROD002
bob.wilson@test.com,myPassword789,PROD001
alice.brown@test.com,strongPwd321,PROD003
charlie.davis@test.com,testPass654,PROD002
```

## 🔄 Variables et substitutions

### Variables utilisateur
- Format : `{{user.COLONNE}}` 
- Exemple : `{{user.email}}` → valeur de la colonne 'email' du CSV

### Variables environnement
- Format : `{{env.VARIABLE}}`
- Exemple : `{{env.baseUrl}}` → valeur dans variables.json > environment

### Variables extraites
- Extraites des réponses précédentes via JSONPath
- Exemple : `"authToken": "$.data.token"` puis `{{authToken}}`

## 🧠 Validation intelligente

L'outil analyse automatiquement la cohérence entre les fichiers :
- ✅ Vérifie que les variables `{{user.xxx}}` existent dans users.csv
- ✅ Vérifie que les variables `{{env.xxx}}` existent dans variables.json
- ✅ Valide la cohérence entre mode et fichiers fournis
- ✅ Messages d'erreur précis en cas d'incohérence

## 🚀 Utilisation

1. **Accéder à l'interface** : http://votre-serveur:3000
2. **Uploader les fichiers** : Glissez-déposez scenario.json et variables.json (users.csv optionnel)
3. **Validation automatique** : L'outil analyse la cohérence des fichiers
4. **Lancer le test** : Cliquez sur "Lancer le test de charge"

## 📊 Statut du projet

### ✅ Sprint 1 - TERMINÉ
- Upload et validation des fichiers
- Analyse intelligente des variables
- Interface utilisateur complète

### 🚧 Sprint 2 - EN COURS
- Exécution des tests avec Workers Go
- Métriques temps réel
- Mode Virtual Users (VUs)

## 📜 Licence

MIT
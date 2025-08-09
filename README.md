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

````

## 🛠 Installation rapide
```bash
git clone <url-du-repo>
cd <nom-du-repo>
docker compose up
````

## 📜 Licence

MIT

````

3. Sauvegarde le fichier et fais :  
```bash
git add README.md
git commit -m "docs: add base README"
git push
````
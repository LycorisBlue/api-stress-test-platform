package main

import (
   "encoding/json"
   "fmt"
   "log"
   "net/http"
   "os"
   "path/filepath"
   "strconv"
   "time"
)

// APIRequest représente une requête d'exécution de test reçue de l'orchestrateur
type APIRequest struct {
   TestID    string    `json:"test_id"`
   Config    TestConfig `json:"config"`
   Timestamp time.Time `json:"timestamp"`
}

// APIResponse représente la réponse envoyée à l'orchestrateur
type APIResponse struct {
   Status    string      `json:"status"`
   Message   string      `json:"message"`
   TestID    string      `json:"test_id,omitempty"`
   Summary   *TestSummary `json:"summary,omitempty"`
   ReportPath string     `json:"report_path,omitempty"`
   Error     string      `json:"error,omitempty"`
}

// HealthResponse représente la réponse du endpoint /health
type HealthResponse struct {
   Status    string `json:"status"`
   Component string `json:"component"`
   Timestamp string `json:"timestamp"`
   Version   string `json:"version"`
}

// Configuration globale du worker
var (
   serverPort     = getEnvOrDefault("WORKER_PORT", "8090")
   orchestratorURL = getEnvOrDefault("ORCHESTRATOR_URL", "http://orchestrator:8080")
   workerVersion  = "1.0.0"
)

func main() {
   log.Printf("🚀 Démarrage du Worker Go v%s", workerVersion)
   log.Printf("📡 Port d'écoute: %s", serverPort)
   log.Printf("🎯 Orchestrateur: %s", orchestratorURL)
   
   // Initialiser les répertoires de travail
   if err := initializeDirectories(); err != nil {
   	log.Fatalf("❌ Erreur d'initialisation: %v", err)
   }
   
   // Configurer les routes HTTP
   setupRoutes()
   
   // Démarrer le serveur HTTP
   log.Printf("✅ Worker prêt - Listening on :%s", serverPort)
   if err := http.ListenAndServe(":"+serverPort, nil); err != nil {
   	log.Fatalf("❌ Erreur serveur HTTP: %v", err)
   }
}

// setupRoutes configure toutes les routes HTTP du worker
func setupRoutes() {
   http.HandleFunc("/health", handleHealth)
   http.HandleFunc("/execute", handleExecuteTest)
   http.HandleFunc("/status", handleStatus)
   http.HandleFunc("/reports", handleListReports)
   http.HandleFunc("/reports/", handleGetReport)
   http.HandleFunc("/cleanup", handleCleanup)
}

// handleHealth endpoint de santé pour les health checks
func handleHealth(w http.ResponseWriter, r *http.Request) {
   if r.Method != http.MethodGet {
   	http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
   	return
   }
   
   response := HealthResponse{
   	Status:    "ok",
   	Component: "worker",
   	Timestamp: time.Now().Format(time.RFC3339),
   	Version:   workerVersion,
   }
   
   w.Header().Set("Content-Type", "application/json")
   json.NewEncoder(w).Encode(response)
}

// handleExecuteTest endpoint principal pour exécuter un test
func handleExecuteTest(w http.ResponseWriter, r *http.Request) {
   if r.Method != http.MethodPost {
   	http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
   	return
   }
   
   log.Printf("📨 Réception d'une demande d'exécution de test")
   
   // Lire et parser la requête
   var apiRequest APIRequest
   if err := json.NewDecoder(r.Body).Decode(&apiRequest); err != nil {
   	log.Printf("❌ Erreur de parsing de la requête: %v", err)
   	respondWithError(w, "Erreur de parsing de la requête", http.StatusBadRequest)
   	return
   }
   
   // Valider la configuration reçue
   if err := validateTestConfig(apiRequest.Config); err != nil {
   	log.Printf("❌ Configuration invalide: %v", err)
   	respondWithError(w, fmt.Sprintf("Configuration invalide: %v", err), http.StatusBadRequest)
   	return
   }
   
   log.Printf("✅ Configuration validée - Mode: %s, Scénario: %s", 
   	apiRequest.Config.Mode, apiRequest.Config.Scenario.Name)
   
   // Créer le collecteur de métriques
   collector := NewMetricsCollector()
   
   // Exécuter le test
   log.Printf("🏃 Démarrage de l'exécution du test %s", apiRequest.TestID)
   testResult := ExecuteTest(apiRequest.Config, collector)
   
   // Générer et sauvegarder le rapport
   reportPath, err := SaveTestReport(testResult, apiRequest.Config, collector)
   if err != nil {
   	log.Printf("⚠️ Erreur de sauvegarde du rapport: %v", err)
   	// Continuer malgré l'erreur de sauvegarde
   } else {
   	log.Printf("📄 Rapport sauvé: %s", reportPath)
   }
   
   // Générer le résumé pour la réponse
   summary := GenerateTestSummary(testResult, collector)
   
   // Préparer la réponse
   response := APIResponse{
   	Status:     "completed",
   	Message:    fmt.Sprintf("Test terminé avec le statut: %s", testResult.Status),
   	TestID:     apiRequest.TestID,
   	Summary:    &summary,
   	ReportPath: reportPath,
   }
   
   if testResult.Status == "failed" {
   	response.Error = testResult.ErrorMsg
   }
   
   // Log du résultat
   log.Printf("✅ Test %s terminé - Statut: %s, Requêtes: %d, RPS: %.1f", 
   	apiRequest.TestID, testResult.Status, summary.TotalRequests, summary.RequestsPerSecond)
   
   // Envoyer la réponse
   w.Header().Set("Content-Type", "application/json")
   json.NewEncoder(w).Encode(response)
}

// handleStatus endpoint pour obtenir le statut actuel du worker
func handleStatus(w http.ResponseWriter, r *http.Request) {
   if r.Method != http.MethodGet {
   	http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
   	return
   }
   
   // Compter les rapports disponibles
   reports, _ := ListTestReports()
   
   status := map[string]interface{}{
   	"status":         "ready",
   	"component":      "worker",
   	"version":        workerVersion,
   	"timestamp":      time.Now().Format(time.RFC3339),
   	"reports_count":  len(reports),
   	"orchestrator":   orchestratorURL,
   }
   
   w.Header().Set("Content-Type", "application/json")
   json.NewEncoder(w).Encode(status)
}

// handleListReports endpoint pour lister tous les rapports disponibles
func handleListReports(w http.ResponseWriter, r *http.Request) {
   if r.Method != http.MethodGet {
   	http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
   	return
   }
   
   reports, err := ListTestReports()
   if err != nil {
   	log.Printf("❌ Erreur de listage des rapports: %v", err)
   	respondWithError(w, "Erreur de listage des rapports", http.StatusInternalServerError)
   	return
   }
   
   response := map[string]interface{}{
   	"status":  "success",
   	"count":   len(reports),
   	"reports": reports,
   }
   
   w.Header().Set("Content-Type", "application/json")
   json.NewEncoder(w).Encode(response)
}

// handleGetReport endpoint pour récupérer un rapport spécifique
func handleGetReport(w http.ResponseWriter, r *http.Request) {
   if r.Method != http.MethodGet {
   	http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
   	return
   }
   
   // Extraire le nom du fichier de l'URL (/reports/filename.json)
   filename := filepath.Base(r.URL.Path)
   if filename == "reports" || filename == "" {
   	respondWithError(w, "Nom de fichier manquant", http.StatusBadRequest)
   	return
   }
   
   // Construire le chemin complet
   filePath := filepath.Join(GetReportsDirectory(), filename)
   
   // Vérifier que le fichier existe
   if _, err := os.Stat(filePath); os.IsNotExist(err) {
   	respondWithError(w, "Rapport non trouvé", http.StatusNotFound)
   	return
   }
   
   // Lire et retourner le fichier
   data, err := os.ReadFile(filePath)
   if err != nil {
   	log.Printf("❌ Erreur de lecture du rapport %s: %v", filename, err)
   	respondWithError(w, "Erreur de lecture du rapport", http.StatusInternalServerError)
   	return
   }
   
   w.Header().Set("Content-Type", "application/json")
   w.Write(data)
}

// handleCleanup endpoint pour nettoyer les anciens rapports
func handleCleanup(w http.ResponseWriter, r *http.Request) {
   if r.Method != http.MethodPost {
   	http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
   	return
   }
   
   // Lire le paramètre de durée (par défaut 7 jours)
   maxAgeDays := 7
   if daysStr := r.URL.Query().Get("days"); daysStr != "" {
   	if days, err := strconv.Atoi(daysStr); err == nil && days > 0 {
   		maxAgeDays = days
   	}
   }
   
   maxAge := time.Duration(maxAgeDays) * 24 * time.Hour
   
   log.Printf("🧹 Nettoyage des rapports plus anciens que %d jours", maxAgeDays)
   
   if err := CleanupOldReports(maxAge); err != nil {
   	log.Printf("❌ Erreur de nettoyage: %v", err)
   	respondWithError(w, "Erreur de nettoyage", http.StatusInternalServerError)
   	return
   }
   
   response := map[string]interface{}{
   	"status":  "success",
   	"message": fmt.Sprintf("Nettoyage effectué (fichiers > %d jours)", maxAgeDays),
   }
   
   w.Header().Set("Content-Type", "application/json")
   json.NewEncoder(w).Encode(response)
}

// validateTestConfig valide la configuration d'un test
func validateTestConfig(config TestConfig) error {
   // Vérifier le mode
   if config.Mode != "users" && config.Mode != "requests" {
   	return fmt.Errorf("mode invalide: %s (doit être 'users' ou 'requests')", config.Mode)
   }
   
   // Vérifier les paramètres selon le mode
   if config.Mode == "users" && config.VirtualUsers <= 0 {
   	return fmt.Errorf("virtualUsers doit être > 0 en mode 'users'")
   }
   
   if config.Mode == "requests" && config.TotalRequests <= 0 {
   	return fmt.Errorf("totalRequests doit être > 0 en mode 'requests'")
   }
   
   // Vérifier le scénario
   if config.Scenario.Name == "" {
   	return fmt.Errorf("nom du scénario manquant")
   }
   
   if len(config.Scenario.Steps) == 0 {
   	return fmt.Errorf("le scénario doit avoir au moins une étape")
   }
   
   // Vérifier chaque étape
   for i, step := range config.Scenario.Steps {
   	if step.Name == "" {
   		return fmt.Errorf("étape %d: nom manquant", i+1)
   	}
   	if step.Method == "" {
   		return fmt.Errorf("étape %d (%s): méthode HTTP manquante", i+1, step.Name)
   	}
   	if step.URL == "" {
   		return fmt.Errorf("étape %d (%s): URL manquante", i+1, step.Name)
   	}
   }
   
   return nil
}

// respondWithError envoie une réponse d'erreur formatée
func respondWithError(w http.ResponseWriter, message string, statusCode int) {
   response := APIResponse{
   	Status:  "error",
   	Message: message,
   	Error:   message,
   }
   
   w.Header().Set("Content-Type", "application/json")
   w.WriteHeader(statusCode)
   json.NewEncoder(w).Encode(response)
}

// initializeDirectories crée les répertoires de travail nécessaires
func initializeDirectories() error {
   dirs := []string{
   	"/tmp/loadtest",
   	"/tmp/loadtest/results",
   	"/tmp/loadtest/config",
   }
   
   for _, dir := range dirs {
   	if err := os.MkdirAll(dir, 0755); err != nil {
   		return fmt.Errorf("impossible de créer le répertoire %s: %w", dir, err)
   	}
   }
   
   log.Printf("📁 Répertoires de travail initialisés")
   return nil
}

// getEnvOrDefault récupère une variable d'environnement ou retourne une valeur par défaut
func getEnvOrDefault(envVar, defaultValue string) string {
   if value := os.Getenv(envVar); value != "" {
   	return value
   }
   return defaultValue
}

// registerWithOrchestrator enregistre ce worker auprès de l'orchestrateur
func registerWithOrchestrator() error {
   // Cette fonction pourrait être utilisée pour un auto-discovery
   // Pour l'instant, on utilise une configuration statique
   log.Printf("🔗 Worker configuré pour communiquer avec %s", orchestratorURL)
   return nil
}

// logRequestDetails affiche les détails d'une requête pour le debugging
func logRequestDetails(r *http.Request) {
   log.Printf("📨 %s %s - Remote: %s, User-Agent: %s", 
   	r.Method, r.URL.Path, r.RemoteAddr, r.Header.Get("User-Agent"))
}

// Fonction d'initialisation appelée au démarrage
func init() {
   // Configuration du logger
   log.SetFlags(log.LstdFlags | log.Lshortfile)
   log.SetPrefix("[WORKER] ")
   
   // Banner de démarrage
   fmt.Println(`
   ╔══════════════════════════════════╗
   ║        Load Test Worker          ║
   ║         Go Version 1.22          ║
   ╚══════════════════════════════════╝
   `)
}
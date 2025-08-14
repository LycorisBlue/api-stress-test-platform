package main

import (
   "bytes"
   "context"
   "encoding/csv"
   "encoding/json"
   "fmt"
   "io"
   "net/http"
   "regexp"
   "strings"
   "sync"
   "time"
)

// TestConfig représente la configuration complète du test
type TestConfig struct {
   Mode         string            `json:"mode"`          // "users" ou "requests"
   VirtualUsers int               `json:"virtualUsers"`  // Pour mode "users"
   TotalRequests int              `json:"totalRequests"` // Pour mode "requests"
   Duration     string            `json:"duration"`      // Durée max (ex: "2m")
   Warmup       string            `json:"warmup"`        // Période de chauffe (ex: "30s")
   Environment  map[string]interface{} `json:"environment"`   // Variables env.xxx
   Scenario     Scenario          `json:"scenario"`      // Scénario à exécuter
   UsersData    []map[string]string // Données CSV chargées
}

// Scenario représente un scénario de test complet
type Scenario struct {
   Name  string         `json:"name"`
   Steps []ScenarioStep `json:"steps"`
}

// ScenarioStep représente une étape du scénario
type ScenarioStep struct {
   Name    string                 `json:"name"`
   Method  string                 `json:"method"`
   URL     string                 `json:"url"`
   Headers map[string]string      `json:"headers,omitempty"`
   Body    map[string]interface{} `json:"body,omitempty"`
   Extract map[string]string      `json:"extract,omitempty"` // Variables à extraire
}

// TestResult contient le résultat final d'un test
type TestResult struct {
   TestID     string    `json:"test_id"`
   Status     string    `json:"status"` // "success", "failed", "timeout"
   StartTime  time.Time `json:"start_time"`
   EndTime    time.Time `json:"end_time"`
   Duration   string    `json:"duration"`
   ErrorMsg   string    `json:"error_msg,omitempty"`
}

// UserSession représente une session utilisateur avec ses variables
type UserSession struct {
   UserData      map[string]string // Données utilisateur du CSV
   ExtractedVars map[string]string // Variables extraites des réponses
   HTTPClient    *http.Client      // Client HTTP réutilisable
}

// ExecuteTest lance l'exécution du test selon la configuration
func ExecuteTest(config TestConfig, collector *MetricsCollector) TestResult {
   testID := fmt.Sprintf("test_%d", time.Now().Unix())
   
   result := TestResult{
   	TestID:    testID,
   	StartTime: time.Now(),
   	Status:    "running",
   }
   
   // Démarrer la collecte des métriques
   collector.StartCollection()
   defer collector.StopCollection()
   
   // Période de warmup si configurée
   if config.Warmup != "" {
   	warmupDuration, err := time.ParseDuration(config.Warmup)
   	if err == nil && warmupDuration > 0 {
   		time.Sleep(warmupDuration)
   	}
   }
   
   // Exécution selon le mode
   var err error
   switch config.Mode {
   case "users":
   	err = executeUsersMode(config, collector)
   case "requests":
   	err = executeRequestsMode(config, collector)
   default:
   	err = fmt.Errorf("mode non supporté: %s", config.Mode)
   }
   
   // Finaliser le résultat
   result.EndTime = time.Now()
   result.Duration = result.EndTime.Sub(result.StartTime).String()
   
   if err != nil {
   	result.Status = "failed"
   	result.ErrorMsg = err.Error()
   } else {
   	result.Status = "success"
   }
   
   return result
}

// executeUsersMode exécute le test en mode utilisateurs virtuels
func executeUsersMode(config TestConfig, collector *MetricsCollector) error {
   if config.VirtualUsers <= 0 {
   	return fmt.Errorf("nombre d'utilisateurs virtuels invalide: %d", config.VirtualUsers)
   }
   
   // Parse de la durée du test
   duration := 2 * time.Minute // Valeur par défaut
   if config.Duration != "" {
   	if parsed, err := time.ParseDuration(config.Duration); err == nil {
   		duration = parsed
   	}
   }
   
   // Context avec timeout pour limiter la durée
   ctx, cancel := context.WithTimeout(context.Background(), duration)
   defer cancel()
   
   // WaitGroup pour attendre toutes les goroutines
   var wg sync.WaitGroup
   
   // Lancer les utilisateurs virtuels
   for i := 0; i < config.VirtualUsers; i++ {
   	wg.Add(1)
   	
   	// Créer une session utilisateur
   	userSession := createUserSession(config, i)
   	
   	go func(session *UserSession) {
   		defer wg.Done()
   		executeUserSession(ctx, config, session, collector)
   	}(userSession)
   }
   
   // Attendre la fin de tous les utilisateurs
   wg.Wait()
   
   return nil
}

// executeRequestsMode exécute le test en mode nombre total de requêtes
func executeRequestsMode(config TestConfig, collector *MetricsCollector) error {
   if config.TotalRequests <= 0 {
   	return fmt.Errorf("nombre total de requêtes invalide: %d", config.TotalRequests)
   }
   
   // Créer une session utilisateur unique pour ce mode
   userSession := createUserSession(config, 0)
   
   // Exécuter les requêtes séquentiellement
   for i := 0; i < config.TotalRequests; i++ {
   	err := executeScenarioOnce(config, userSession, collector)
   	if err != nil {
   		// Log l'erreur mais continue l'exécution
   		fmt.Printf("Erreur lors de l'exécution de la requête %d: %v\n", i+1, err)
   	}
   }
   
   return nil
}

// createUserSession crée une nouvelle session utilisateur
func createUserSession(config TestConfig, userIndex int) *UserSession {
   session := &UserSession{
   	UserData:      make(map[string]string),
   	ExtractedVars: make(map[string]string),
   	HTTPClient: &http.Client{
   		Timeout: 10 * time.Second, // Timeout par défaut
   	},
   }
   
   // Assigner les données utilisateur du CSV si disponibles
   if len(config.UsersData) > 0 {
   	// Utiliser l'index modulo pour répartir les utilisateurs
   	userData := config.UsersData[userIndex%len(config.UsersData)]
   	session.UserData = userData
   }
   
   return session
}

// executeUserSession exécute une session utilisateur en boucle
func executeUserSession(ctx context.Context, config TestConfig, session *UserSession, collector *MetricsCollector) {
   for {
   	select {
   	case <-ctx.Done():
   		// Timeout atteint, arrêter cette session
   		return
   	default:
   		// Exécuter le scénario une fois
   		err := executeScenarioOnce(config, session, collector)
   		if err != nil {
   			// Log l'erreur mais continue
   			fmt.Printf("Erreur dans session utilisateur: %v\n", err)
   		}
   		
   		// Petite pause entre les itérations pour éviter de surcharger
   		time.Sleep(100 * time.Millisecond)
   	}
   }
}

// executeScenarioOnce exécute le scénario complet une fois
func executeScenarioOnce(config TestConfig, session *UserSession, collector *MetricsCollector) error {
   for _, step := range config.Scenario.Steps {
   	err := executeStep(step, config, session, collector)
   	if err != nil {
   		return fmt.Errorf("erreur à l'étape '%s': %w", step.Name, err)
   	}
   }
   return nil
}

// executeStep exécute une étape individuelle du scénario
func executeStep(step ScenarioStep, config TestConfig, session *UserSession, collector *MetricsCollector) error {
   startTime := time.Now()
   
   // Substituer les variables dans l'URL
   url := substituteVariables(step.URL, config, session)
   
   // Préparer le body de la requête si présent
   var bodyReader io.Reader
   if step.Body != nil {
   	// Substituer les variables dans le body
   	bodyWithVars := substituteVariablesInBody(step.Body, config, session)
   	
   	bodyBytes, err := json.Marshal(bodyWithVars)
   	if err != nil {
   		collector.AddRequest(step.Name, time.Since(startTime), 0, "Erreur de sérialisation JSON")
   		return fmt.Errorf("erreur de sérialisation du body: %w", err)
   	}
   	
   	bodyReader = bytes.NewReader(bodyBytes)
   }
   
   // Créer la requête HTTP
   req, err := http.NewRequest(step.Method, url, bodyReader)
   if err != nil {
   	collector.AddRequest(step.Name, time.Since(startTime), 0, "Erreur de création de requête")
   	return fmt.Errorf("erreur de création de requête: %w", err)
   }
   
   // Ajouter les headers avec substitution de variables
   for key, value := range step.Headers {
   	substitutedValue := substituteVariables(value, config, session)
   	req.Header.Set(key, substitutedValue)
   }
   
   // Exécuter la requête
   resp, err := session.HTTPClient.Do(req)
   duration := time.Since(startTime)
   
   if err != nil {
   	collector.AddRequest(step.Name, duration, 0, err.Error())
   	return fmt.Errorf("erreur de requête HTTP: %w", err)
   }
   defer resp.Body.Close()
   
   // Lire la réponse
   respBody, err := io.ReadAll(resp.Body)
   if err != nil {
   	collector.AddRequest(step.Name, duration, resp.StatusCode, "Erreur de lecture de réponse")
   	return fmt.Errorf("erreur de lecture de réponse: %w", err)
   }
   
   // Enregistrer la métrique
   collector.AddRequest(step.Name, duration, resp.StatusCode, "")
   
   // Extraire les variables si configuré
   if step.Extract != nil {
   	err = extractVariables(step.Extract, respBody, session)
   	if err != nil {
   		fmt.Printf("Erreur d'extraction de variables pour l'étape '%s': %v\n", step.Name, err)
   	}
   }
   
   return nil
}

// substituteVariables remplace les variables {{xxx}} dans une chaîne
func substituteVariables(text string, config TestConfig, session *UserSession) string {
   // Regex pour capturer {{variable}}
   re := regexp.MustCompile(`\{\{([^}]+)\}\}`)
   
   return re.ReplaceAllStringFunc(text, func(match string) string {
   	// Extraire le nom de la variable (sans les {{}})
   	varName := strings.Trim(match, "{}")
   	
   	// Variables utilisateur (user.xxx)
   	if strings.HasPrefix(varName, "user.") {
   		fieldName := strings.TrimPrefix(varName, "user.")
   		if value, exists := session.UserData[fieldName]; exists {
   			return value
   		}
   	}
   	
   	// Variables d'environnement (env.xxx)
   	if strings.HasPrefix(varName, "env.") {
   		fieldName := strings.TrimPrefix(varName, "env.")
		if value, exists := config.Environment[fieldName]; exists {
			return fmt.Sprintf("%v", value)  // Convertit en string
		}
   	}
   	
   	// Variables extraites
   	if value, exists := session.ExtractedVars[varName]; exists {
   		return value
   	}
   	
   	// Si la variable n'est pas trouvée, retourner la variable originale
   	return match
   })
}

// substituteVariablesInBody substitue les variables dans un body JSON
func substituteVariablesInBody(body map[string]interface{}, config TestConfig, session *UserSession) map[string]interface{} {
   result := make(map[string]interface{})
   
   for key, value := range body {
   	switch v := value.(type) {
   	case string:
   		result[key] = substituteVariables(v, config, session)
   	case map[string]interface{}:
   		result[key] = substituteVariablesInBody(v, config, session)
   	default:
   		result[key] = value
   	}
   }
   
   return result
}

// extractVariables extrait des variables de la réponse selon la configuration
func extractVariables(extractConfig map[string]string, responseBody []byte, session *UserSession) error {
   // Parse de la réponse JSON
   var jsonResponse map[string]interface{}
   if err := json.Unmarshal(responseBody, &jsonResponse); err != nil {
   	return fmt.Errorf("impossible de parser la réponse JSON: %w", err)
   }
   
   // Pour chaque variable à extraire
   for varName, jsonPath := range extractConfig {
   	value, err := extractJSONPath(jsonResponse, jsonPath)
   	if err != nil {
   		fmt.Printf("Impossible d'extraire %s avec le chemin %s: %v\n", varName, jsonPath, err)
   		continue
   	}
   	
   	// Stocker la variable extraite
   	session.ExtractedVars[varName] = fmt.Sprintf("%v", value)
   }
   
   return nil
}

// extractJSONPath extrait une valeur d'un JSON avec un chemin simple (ex: "$.data.token")
func extractJSONPath(data map[string]interface{}, path string) (interface{}, error) {
	// Implémentation simplifiée de JSONPath
	// Supporte uniquement les chemins simples comme "$.data.token"
	
	if !strings.HasPrefix(path, "$.") {
		return nil, fmt.Errorf("chemin JSONPath invalide: %s", path)
	}
	
	// Enlever le "$." du début
	path = strings.TrimPrefix(path, "$.")
	
	// Séparer les parties du chemin
	parts := strings.Split(path, ".")
	
	var current interface{} = data  // ✅ Déclaration corrigée
	for _, part := range parts {
		if current == nil {
			return nil, fmt.Errorf("chemin non trouvé: %s", part)
		}
		
		// Convertir current en map si possible
		if currentMap, ok := current.(map[string]interface{}); ok {
			current = currentMap[part]  // ✅ Maintenant correct
		} else {
			return nil, fmt.Errorf("impossible de naviguer dans %s", part)
		}
	}
	
	return current, nil
}

// LoadUsersFromCSV charge les données utilisateurs depuis un contenu CSV
func LoadUsersFromCSV(csvContent string) ([]map[string]string, error) {
   reader := csv.NewReader(strings.NewReader(csvContent))
   
   // Lire les en-têtes
   headers, err := reader.Read()
   if err != nil {
   	return nil, fmt.Errorf("erreur de lecture des en-têtes CSV: %w", err)
   }
   
   // Nettoyer les en-têtes (supprimer les espaces)
   for i, header := range headers {
   	headers[i] = strings.TrimSpace(header)
   }
   
   var users []map[string]string
   
   // Lire chaque ligne
   for {
   	record, err := reader.Read()
   	if err == io.EOF {
   		break
   	}
   	if err != nil {
   		return nil, fmt.Errorf("erreur de lecture d'une ligne CSV: %w", err)
   	}
   	
   	// Créer un map pour cet utilisateur
   	user := make(map[string]string)
   	for i, value := range record {
   		if i < len(headers) {
   			user[headers[i]] = strings.TrimSpace(value)
   		}
   	}
   	
   	users = append(users, user)
   }
   
   return users, nil
}
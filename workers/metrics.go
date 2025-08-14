package main

import (
   "math"
   "sort"
   "sync"
   "time"
)

// RequestMetric représente une mesure individuelle d'une requête
type RequestMetric struct {
   StepName     string        // Nom de l'étape du scénario
   Duration     time.Duration // Temps de réponse
   StatusCode   int           // Code HTTP de retour
   Success      bool          // true si 2xx, false sinon
   Timestamp    time.Time     // Moment de la requête
   ErrorMessage string        // Message d'erreur si échec
}

// StepMetrics contient les métriques agrégées pour une étape
type StepMetrics struct {
   StepName           string  `json:"step_name"`
   TotalRequests      int     `json:"total_requests"`
   SuccessfulRequests int     `json:"successful_requests"`
   FailedRequests     int     `json:"failed_requests"`
   ErrorRate          float64 `json:"error_rate"`
   AvgResponseTimeMs  float64 `json:"avg_response_time_ms"`
   P95ResponseTimeMs  float64 `json:"p95_response_time_ms"`
   P99ResponseTimeMs  float64 `json:"p99_response_time_ms"`
   MinResponseTimeMs  float64 `json:"min_response_time_ms"`
   MaxResponseTimeMs  float64 `json:"max_response_time_ms"`
}

// GlobalMetrics contient les métriques globales de tout le test
type GlobalMetrics struct {
   TotalRequests      int     `json:"total_requests"`
   SuccessfulRequests int     `json:"successful_requests"`
   FailedRequests     int     `json:"failed_requests"`
   ErrorRate          float64 `json:"error_rate"`
   RequestsPerSecond  float64 `json:"requests_per_second"`
   AvgResponseTimeMs  float64 `json:"avg_response_time_ms"`
   P95ResponseTimeMs  float64 `json:"p95_response_time_ms"`
   P99ResponseTimeMs  float64 `json:"p99_response_time_ms"`
   MinResponseTimeMs  float64 `json:"min_response_time_ms"`
   MaxResponseTimeMs  float64 `json:"max_response_time_ms"`
}

// ErrorSummary résume les erreurs par type
type ErrorSummary struct {
   StepName   string  `json:"step_name"`
   ErrorType  string  `json:"error_type"`
   Count      int     `json:"count"`
   Percentage float64 `json:"percentage"`
}

// MetricsCollector collecte et calcule les métriques en temps réel
type MetricsCollector struct {
   mutex       sync.RWMutex    // Protection thread-safe
   requests    []RequestMetric // Toutes les requêtes individuelles
   startTime   time.Time       // Début du test
   endTime     time.Time       // Fin du test
   isRunning   bool            // Indique si le test est en cours
}

// NewMetricsCollector crée un nouveau collecteur de métriques
func NewMetricsCollector() *MetricsCollector {
   return &MetricsCollector{
   	requests:  make([]RequestMetric, 0),
   	isRunning: false,
   }
}

// StartCollection démarre la collecte des métriques
func (mc *MetricsCollector) StartCollection() {
   mc.mutex.Lock()
   defer mc.mutex.Unlock()
   
   mc.startTime = time.Now()
   mc.isRunning = true
   mc.requests = make([]RequestMetric, 0) // Reset des métriques précédentes
}

// StopCollection arrête la collecte des métriques
func (mc *MetricsCollector) StopCollection() {
   mc.mutex.Lock()
   defer mc.mutex.Unlock()
   
   mc.endTime = time.Now()
   mc.isRunning = false
}

// AddRequest ajoute une nouvelle mesure de requête (thread-safe)
func (mc *MetricsCollector) AddRequest(stepName string, duration time.Duration, statusCode int, errorMsg string) {
   mc.mutex.Lock()
   defer mc.mutex.Unlock()
   
   // Détermine si la requête est un succès (codes 2xx)
   success := statusCode >= 200 && statusCode < 300
   
   metric := RequestMetric{
   	StepName:     stepName,
   	Duration:     duration,
   	StatusCode:   statusCode,
   	Success:      success,
   	Timestamp:    time.Now(),
   	ErrorMessage: errorMsg,
   }
   
   mc.requests = append(mc.requests, metric)
}

// GetCurrentRPS calcule le RPS actuel (thread-safe)
func (mc *MetricsCollector) GetCurrentRPS() float64 {
   mc.mutex.RLock()
   defer mc.mutex.RUnlock()
   
   if !mc.isRunning || len(mc.requests) == 0 {
   	return 0
   }
   
   elapsed := time.Since(mc.startTime).Seconds()
   if elapsed == 0 {
   	return 0
   }
   
   return float64(len(mc.requests)) / elapsed
}

// GetTotalRequests retourne le nombre total de requêtes (thread-safe)
func (mc *MetricsCollector) GetTotalRequests() int {
   mc.mutex.RLock()
   defer mc.mutex.RUnlock()
   
   return len(mc.requests)
}

// calculatePercentile calcule un percentile donné sur une slice de durées
func calculatePercentile(durations []time.Duration, percentile float64) float64 {
   if len(durations) == 0 {
   	return 0
   }
   
   // Trier les durées
   sort.Slice(durations, func(i, j int) bool {
   	return durations[i] < durations[j]
   })
   
   // Calculer l'index du percentile
   index := percentile * float64(len(durations)-1)
   lower := int(math.Floor(index))
   upper := int(math.Ceil(index))
   
   if lower == upper {
   	return float64(durations[lower].Nanoseconds()) / 1e6 // Conversion en ms
   }
   
   // Interpolation linéaire
   weight := index - float64(lower)
   lowerValue := float64(durations[lower].Nanoseconds()) / 1e6
   upperValue := float64(durations[upper].Nanoseconds()) / 1e6
   
   return lowerValue + weight*(upperValue-lowerValue)
}

// GetStepMetrics calcule les métriques pour une étape spécifique
func (mc *MetricsCollector) GetStepMetrics(stepName string) StepMetrics {
   mc.mutex.RLock()
   defer mc.mutex.RUnlock()
   
   // Filtrer les requêtes pour cette étape
   stepRequests := make([]RequestMetric, 0)
   for _, req := range mc.requests {
   	if req.StepName == stepName {
   		stepRequests = append(stepRequests, req)
   	}
   }
   
   if len(stepRequests) == 0 {
   	return StepMetrics{StepName: stepName}
   }
   
   // Calculs de base
   totalRequests := len(stepRequests)
   successfulRequests := 0
   var totalDuration time.Duration
   durations := make([]time.Duration, 0, totalRequests)
   
   minDuration := stepRequests[0].Duration
   maxDuration := stepRequests[0].Duration
   
   for _, req := range stepRequests {
   	if req.Success {
   		successfulRequests++
   	}
   	
   	totalDuration += req.Duration
   	durations = append(durations, req.Duration)
   	
   	if req.Duration < minDuration {
   		minDuration = req.Duration
   	}
   	if req.Duration > maxDuration {
   		maxDuration = req.Duration
   	}
   }
   
   failedRequests := totalRequests - successfulRequests
   errorRate := float64(failedRequests) / float64(totalRequests)
   avgResponseTime := float64(totalDuration.Nanoseconds()) / float64(totalRequests) / 1e6 // ms
   
   return StepMetrics{
   	StepName:           stepName,
   	TotalRequests:      totalRequests,
   	SuccessfulRequests: successfulRequests,
   	FailedRequests:     failedRequests,
   	ErrorRate:          errorRate,
   	AvgResponseTimeMs:  avgResponseTime,
   	P95ResponseTimeMs:  calculatePercentile(durations, 0.95),
   	P99ResponseTimeMs:  calculatePercentile(durations, 0.99),
   	MinResponseTimeMs:  float64(minDuration.Nanoseconds()) / 1e6,
   	MaxResponseTimeMs:  float64(maxDuration.Nanoseconds()) / 1e6,
   }
}

// GetGlobalMetrics calcule les métriques globales de tout le test
func (mc *MetricsCollector) GetGlobalMetrics() GlobalMetrics {
   mc.mutex.RLock()
   defer mc.mutex.RUnlock()
   
   if len(mc.requests) == 0 {
   	return GlobalMetrics{}
   }
   
   totalRequests := len(mc.requests)
   successfulRequests := 0
   var totalDuration time.Duration
   durations := make([]time.Duration, 0, totalRequests)
   
   minDuration := mc.requests[0].Duration
   maxDuration := mc.requests[0].Duration
   
   for _, req := range mc.requests {
   	if req.Success {
   		successfulRequests++
   	}
   	
   	totalDuration += req.Duration
   	durations = append(durations, req.Duration)
   	
   	if req.Duration < minDuration {
   		minDuration = req.Duration
   	}
   	if req.Duration > maxDuration {
   		maxDuration = req.Duration
   	}
   }
   
   failedRequests := totalRequests - successfulRequests
   errorRate := float64(failedRequests) / float64(totalRequests)
   avgResponseTime := float64(totalDuration.Nanoseconds()) / float64(totalRequests) / 1e6 // ms
   
   // Calcul du RPS sur la durée totale
   var testDuration time.Duration
   if mc.isRunning {
   	testDuration = time.Since(mc.startTime)
   } else {
   	testDuration = mc.endTime.Sub(mc.startTime)
   }
   
   rps := float64(totalRequests) / testDuration.Seconds()
   
   return GlobalMetrics{
   	TotalRequests:      totalRequests,
   	SuccessfulRequests: successfulRequests,
   	FailedRequests:     failedRequests,
   	ErrorRate:          errorRate,
   	RequestsPerSecond:  rps,
   	AvgResponseTimeMs:  avgResponseTime,
   	P95ResponseTimeMs:  calculatePercentile(durations, 0.95),
   	P99ResponseTimeMs:  calculatePercentile(durations, 0.99),
   	MinResponseTimeMs:  float64(minDuration.Nanoseconds()) / 1e6,
   	MaxResponseTimeMs:  float64(maxDuration.Nanoseconds()) / 1e6,
   }
}

// GetErrorSummary retourne un résumé des erreurs par type et étape
func (mc *MetricsCollector) GetErrorSummary() []ErrorSummary {
   mc.mutex.RLock()
   defer mc.mutex.RUnlock()
   
   // Compter les erreurs par type et étape
   errorCounts := make(map[string]map[string]int) // stepName -> errorType -> count
   
   for _, req := range mc.requests {
   	if !req.Success {
   		if errorCounts[req.StepName] == nil {
   			errorCounts[req.StepName] = make(map[string]int)
   		}
   		
   		// Détermine le type d'erreur basé sur le code de statut
   		errorType := "UNKNOWN"
   		if req.StatusCode >= 400 && req.StatusCode < 500 {
   			errorType = "CLIENT_ERROR"
   		} else if req.StatusCode >= 500 {
   			errorType = "SERVER_ERROR"
   		} else if req.ErrorMessage != "" {
   			errorType = "NETWORK_ERROR"
   		}
   		
   		errorCounts[req.StepName][errorType]++
   	}
   }
   
   // Convertir en slice avec pourcentages
   var summary []ErrorSummary
   totalRequests := len(mc.requests)
   
   for stepName, errorTypes := range errorCounts {
   	for errorType, count := range errorTypes {
   		percentage := float64(count) / float64(totalRequests) * 100
   		
   		summary = append(summary, ErrorSummary{
   			StepName:   stepName,
   			ErrorType:  errorType,
   			Count:      count,
   			Percentage: percentage,
   		})
   	}
   }
   
   return summary
}

// GetUniqueStepNames retourne la liste des noms d'étapes uniques
func (mc *MetricsCollector) GetUniqueStepNames() []string {
   mc.mutex.RLock()
   defer mc.mutex.RUnlock()
   
   stepNamesMap := make(map[string]bool)
   for _, req := range mc.requests {
   	stepNamesMap[req.StepName] = true
   }
   
   stepNames := make([]string, 0, len(stepNamesMap))
   for stepName := range stepNamesMap {
   	stepNames = append(stepNames, stepName)
   }
   
   return stepNames
}
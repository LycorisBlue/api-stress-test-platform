package main

import (
   "encoding/json"
   "fmt"
   "os"
   "path/filepath"
   "time"
)

// TestReport représente la structure complète d'un rapport de test
type TestReport struct {
   TestID         string                 `json:"test_id"`
   Mode           string                 `json:"mode"`
   Status         string                 `json:"status"`
   Config         TestConfigSummary      `json:"config"`
   Execution      ExecutionSummary       `json:"execution"`
   GlobalMetrics  GlobalMetrics          `json:"global_metrics"`
   StepsMetrics   []StepMetrics          `json:"steps_metrics"`
   Errors         []ErrorSummary         `json:"errors"`
   ErrorMessage   string                 `json:"error_message,omitempty"`
}

// TestConfigSummary contient un résumé de la configuration utilisée
type TestConfigSummary struct {
   Mode            string `json:"mode"`
   VirtualUsers    int    `json:"virtual_users,omitempty"`
   TotalRequests   int    `json:"total_requests,omitempty"`
   Duration        string `json:"duration,omitempty"`
   Warmup          string `json:"warmup,omitempty"`
   ScenarioName    string `json:"scenario_name"`
   StepsCount      int    `json:"steps_count"`
   UsersDataCount  int    `json:"users_data_count"`
}

// ExecutionSummary contient les informations d'exécution du test
type ExecutionSummary struct {
   StartTime         time.Time `json:"start_time"`
   EndTime           time.Time `json:"end_time"`
   ActualDuration    string    `json:"actual_duration"`
   WarmupDuration    string    `json:"warmup_duration,omitempty"`
   RequestsCompleted int       `json:"requests_completed"`
   RequestsPlanned   int       `json:"requests_planned,omitempty"`
   CompletionRate    float64   `json:"completion_rate,omitempty"`
   FailureReason     string    `json:"failure_reason,omitempty"`
}

// TestSummary représente un résumé condensé pour l'affichage rapide
type TestSummary struct {
   TestID           string  `json:"test_id"`
   Status           string  `json:"status"`
   Duration         string  `json:"duration"`
   TotalRequests    int     `json:"total_requests"`
   SuccessRate      float64 `json:"success_rate"`
   AvgResponseTime  float64 `json:"avg_response_time_ms"`
   P95ResponseTime  float64 `json:"p95_response_time_ms"`
   RequestsPerSecond float64 `json:"requests_per_second"`
   ErrorCount       int     `json:"error_count"`
   Message          string  `json:"message"`
}

// PerformanceAnalysis contient une analyse des performances
type PerformanceAnalysis struct {
   OverallStatus    string   `json:"overall_status"`    // "excellent", "good", "warning", "critical"
   Recommendations  []string `json:"recommendations"`
   ThresholdAlerts  []string `json:"threshold_alerts"`
   BottleneckSteps  []string `json:"bottleneck_steps"`
}

// GenerateTestReport génère le rapport complet d'un test
func GenerateTestReport(testResult TestResult, config TestConfig, collector *MetricsCollector) TestReport {
   // Métriques globales
   globalMetrics := collector.GetGlobalMetrics()
   
   // Métriques par étape
   stepNames := collector.GetUniqueStepNames()
   stepsMetrics := make([]StepMetrics, 0, len(stepNames))
   for _, stepName := range stepNames {
   	stepMetric := collector.GetStepMetrics(stepName)
   	stepsMetrics = append(stepsMetrics, stepMetric)
   }
   
   // Résumé des erreurs
   errorSummary := collector.GetErrorSummary()
   
   // Configuration résumée
   configSummary := TestConfigSummary{
   	Mode:           config.Mode,
   	VirtualUsers:   config.VirtualUsers,
   	TotalRequests:  config.TotalRequests,
   	Duration:       config.Duration,
   	Warmup:         config.Warmup,
   	ScenarioName:   config.Scenario.Name,
   	StepsCount:     len(config.Scenario.Steps),
   	UsersDataCount: len(config.UsersData),
   }
   
   // Informations d'exécution
   executionSummary := ExecutionSummary{
   	StartTime:         testResult.StartTime,
   	EndTime:           testResult.EndTime,
   	ActualDuration:    testResult.Duration,
   	WarmupDuration:    config.Warmup,
   	RequestsCompleted: globalMetrics.TotalRequests,
   	FailureReason:     testResult.ErrorMsg,
   }
   
   // Calcul du taux de completion pour le mode requests
   if config.Mode == "requests" && config.TotalRequests > 0 {
   	executionSummary.RequestsPlanned = config.TotalRequests
   	executionSummary.CompletionRate = float64(globalMetrics.TotalRequests) / float64(config.TotalRequests)
   }
   
   return TestReport{
   	TestID:        testResult.TestID,
   	Mode:          config.Mode,
   	Status:        testResult.Status,
   	Config:        configSummary,
   	Execution:     executionSummary,
   	GlobalMetrics: globalMetrics,
   	StepsMetrics:  stepsMetrics,
   	Errors:        errorSummary,
   	ErrorMessage:  testResult.ErrorMsg,
   }
}

// GenerateTestSummary génère un résumé condensé du test
func GenerateTestSummary(testResult TestResult, collector *MetricsCollector) TestSummary {
   globalMetrics := collector.GetGlobalMetrics()
   
   // Calculer le taux de succès
   successRate := 0.0
   if globalMetrics.TotalRequests > 0 {
   	successRate = float64(globalMetrics.SuccessfulRequests) / float64(globalMetrics.TotalRequests) * 100
   }
   
   // Message de statut
   message := generateStatusMessage(testResult.Status, globalMetrics)
   
   return TestSummary{
   	TestID:           testResult.TestID,
   	Status:           testResult.Status,
   	Duration:         testResult.Duration,
   	TotalRequests:    globalMetrics.TotalRequests,
   	SuccessRate:      successRate,
   	AvgResponseTime:  globalMetrics.AvgResponseTimeMs,
   	P95ResponseTime:  globalMetrics.P95ResponseTimeMs,
   	RequestsPerSecond: globalMetrics.RequestsPerSecond,
   	ErrorCount:       globalMetrics.FailedRequests,
   	Message:          message,
   }
}

// generateStatusMessage génère un message de statut descriptif
func generateStatusMessage(status string, metrics GlobalMetrics) string {
   switch status {
   case "success":
   	if metrics.ErrorRate > 0.05 { // Plus de 5% d'erreurs
   		return fmt.Sprintf("Test terminé avec %d erreurs (%.1f%%)", metrics.FailedRequests, metrics.ErrorRate*100)
   	} else if metrics.P95ResponseTimeMs > 500 { // P95 > 500ms
   		return fmt.Sprintf("Test terminé mais performances dégradées (P95: %.0fms)", metrics.P95ResponseTimeMs)
   	} else {
   		return fmt.Sprintf("Test terminé avec succès - %d requêtes, %.1f RPS", metrics.TotalRequests, metrics.RequestsPerSecond)
   	}
   case "failed":
   	return "Test échoué - voir les détails d'erreur"
   case "timeout":
   	return "Test interrompu par timeout"
   default:
   	return "Statut inconnu"
   }
}

// AnalyzePerformance effectue une analyse des performances
func AnalyzePerformance(globalMetrics GlobalMetrics, stepsMetrics []StepMetrics) PerformanceAnalysis {
   analysis := PerformanceAnalysis{
   	Recommendations: make([]string, 0),
   	ThresholdAlerts: make([]string, 0),
   	BottleneckSteps: make([]string, 0),
   }
   
   // Analyse du taux d'erreur global
   if globalMetrics.ErrorRate > 0.10 { // Plus de 10%
   	analysis.OverallStatus = "critical"
   	analysis.ThresholdAlerts = append(analysis.ThresholdAlerts, 
   		fmt.Sprintf("Taux d'erreur critique: %.1f%% (seuil: 10%%)", globalMetrics.ErrorRate*100))
   	analysis.Recommendations = append(analysis.Recommendations, 
   		"Réduire la charge ou corriger les erreurs serveur")
   } else if globalMetrics.ErrorRate > 0.05 { // Plus de 5%
   	analysis.OverallStatus = "warning"
   	analysis.ThresholdAlerts = append(analysis.ThresholdAlerts, 
   		fmt.Sprintf("Taux d'erreur élevé: %.1f%% (seuil: 5%%)", globalMetrics.ErrorRate*100))
   }
   
   // Analyse des temps de réponse
   if globalMetrics.P95ResponseTimeMs > 1000 { // P95 > 1s
   	if analysis.OverallStatus == "" {
   		analysis.OverallStatus = "critical"
   	}
   	analysis.ThresholdAlerts = append(analysis.ThresholdAlerts, 
   		fmt.Sprintf("P95 critique: %.0fms (seuil: 1000ms)", globalMetrics.P95ResponseTimeMs))
   	analysis.Recommendations = append(analysis.Recommendations, 
   		"Optimiser les performances côté serveur")
   } else if globalMetrics.P95ResponseTimeMs > 500 { // P95 > 500ms
   	if analysis.OverallStatus == "" {
   		analysis.OverallStatus = "warning"
   	}
   	analysis.ThresholdAlerts = append(analysis.ThresholdAlerts, 
   		fmt.Sprintf("P95 élevé: %.0fms (seuil: 500ms)", globalMetrics.P95ResponseTimeMs))
   }
   
   // Identifier les étapes les plus lentes (goulots d'étranglement)
   for _, step := range stepsMetrics {
   	if step.P95ResponseTimeMs > globalMetrics.P95ResponseTimeMs*1.5 { // 50% plus lent que la moyenne
   		analysis.BottleneckSteps = append(analysis.BottleneckSteps, step.StepName)
   	}
   }
   
   if len(analysis.BottleneckSteps) > 0 {
   	analysis.Recommendations = append(analysis.Recommendations, 
   		"Optimiser les étapes identifiées comme goulots d'étranglement")
   }
   
   // Déterminer le statut global si pas encore défini
   if analysis.OverallStatus == "" {
   	if globalMetrics.ErrorRate < 0.01 && globalMetrics.P95ResponseTimeMs < 300 {
   		analysis.OverallStatus = "excellent"
   	} else {
   		analysis.OverallStatus = "good"
   	}
   }
   
   return analysis
}

// SaveTestReport sauvegarde le rapport complet dans un fichier JSON
func SaveTestReport(testResult TestResult, config TestConfig, collector *MetricsCollector) (string, error) {
   // Générer le rapport complet
   report := GenerateTestReport(testResult, config, collector)
   
   // Ajouter l'analyse de performance
   analysis := AnalyzePerformance(report.GlobalMetrics, report.StepsMetrics)
   
   // Créer une structure étendue avec l'analyse
   extendedReport := struct {
   	TestReport
   	Analysis PerformanceAnalysis `json:"performance_analysis"`
   }{
   	TestReport: report,
   	Analysis:   analysis,
   }
   
   // Créer le répertoire de destination
   resultsDir := "/tmp/loadtest/results"
   if err := os.MkdirAll(resultsDir, 0755); err != nil {
   	return "", fmt.Errorf("impossible de créer le répertoire %s: %w", resultsDir, err)
   }
   
   // Générer le nom du fichier avec timestamp
   timestamp := time.Now().Format("20060102_150405")
   filename := fmt.Sprintf("%s_%s.json", report.TestID, timestamp)
   filePath := filepath.Join(resultsDir, filename)
   
   // Sérialiser en JSON avec indentation
   jsonData, err := json.MarshalIndent(extendedReport, "", "  ")
   if err != nil {
   	return "", fmt.Errorf("erreur de sérialisation JSON: %w", err)
   }
   
   // Écrire dans le fichier
   if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
   	return "", fmt.Errorf("erreur d'écriture du fichier %s: %w", filePath, err)
   }
   
   return filePath, nil
}

// SaveTestSummary sauvegarde un résumé condensé
func SaveTestSummary(testResult TestResult, collector *MetricsCollector, summaryPath string) error {
   summary := GenerateTestSummary(testResult, collector)
   
   jsonData, err := json.MarshalIndent(summary, "", "  ")
   if err != nil {
   	return fmt.Errorf("erreur de sérialisation du résumé: %w", err)
   }
   
   return os.WriteFile(summaryPath, jsonData, 0644)
}

// LoadTestReport charge un rapport depuis un fichier JSON
func LoadTestReport(filePath string) (TestReport, error) {
   var report TestReport
   
   data, err := os.ReadFile(filePath)
   if err != nil {
   	return report, fmt.Errorf("erreur de lecture du fichier %s: %w", filePath, err)
   }
   
   if err := json.Unmarshal(data, &report); err != nil {
   	return report, fmt.Errorf("erreur de désérialisation JSON: %w", err)
   }
   
   return report, nil
}

// GetReportsDirectory retourne le répertoire où sont stockés les rapports
func GetReportsDirectory() string {
   return "/tmp/loadtest/results"
}

// ListTestReports liste tous les rapports de test disponibles
func ListTestReports() ([]string, error) {
   resultsDir := GetReportsDirectory()
   
   entries, err := os.ReadDir(resultsDir)
   if err != nil {
   	// Si le répertoire n'existe pas, retourner une liste vide
   	if os.IsNotExist(err) {
   		return []string{}, nil
   	}
   	return nil, fmt.Errorf("erreur de lecture du répertoire %s: %w", resultsDir, err)
   }
   
   var reports []string
   for _, entry := range entries {
   	if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
   		reports = append(reports, entry.Name())
   	}
   }
   
   return reports, nil
}

// CleanupOldReports supprime les rapports plus anciens que la durée spécifiée
func CleanupOldReports(maxAge time.Duration) error {
   resultsDir := GetReportsDirectory()
   cutoffTime := time.Now().Add(-maxAge)
   
   entries, err := os.ReadDir(resultsDir)
   if err != nil {
   	return fmt.Errorf("erreur de lecture du répertoire %s: %w", resultsDir, err)
   }
   
   for _, entry := range entries {
   	if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
   		filePath := filepath.Join(resultsDir, entry.Name())
   		
   		info, err := entry.Info()
   		if err != nil {
   			continue // Ignorer les erreurs sur des fichiers individuels
   		}
   		
   		if info.ModTime().Before(cutoffTime) {
   			if err := os.Remove(filePath); err != nil {
   				fmt.Printf("Impossible de supprimer %s: %v\n", filePath, err)
   			}
   		}
   	}
   }
   
   return nil
}
/* eslint-disable react/no-unescaped-entities */
'use client';

import { useState } from 'react';

interface FileState {
  file: File | null;
  status: 'idle' | 'valid' | 'invalid';
  error?: string;
}

interface ValidationResponse {
  status: string;
  message: string;
  errors: string[];
  warnings: string[];
  analysis: {
    variables_found: {
      user: string[];
      env: string[];
      extract: string[];
    };
    csv_columns: string[];
    files_saved: boolean;
  };
}

export default function HomePage() {
  const [files, setFiles] = useState<{
    scenario: FileState;
    variables: FileState;
    users: FileState;
  }>({
    scenario: { file: null, status: 'idle' },
    variables: { file: null, status: 'idle' },
    users: { file: null, status: 'idle' }
  });

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [validationResult, setValidationResult] = useState<ValidationResponse | null>(null);

  const validateFile = (file: File, type: 'scenario' | 'variables' | 'users'): { valid: boolean; error?: string } => {
    // Validation taille (max 10MB)
    if (file.size > 10 * 1024 * 1024) {
      return { valid: false, error: 'Fichier trop volumineux (max 10MB)' };
    }

    // Validation extension
    if (type === 'users' && !file.name.endsWith('.csv')) {
      return { valid: false, error: 'Le fichier users doit être un CSV' };
    }

    if ((type === 'scenario' || type === 'variables') && !file.name.endsWith('.json')) {
      return { valid: false, error: 'Le fichier doit être un JSON' };
    }

    return { valid: true };
  };

  const handleFileUpload = (file: File, type: 'scenario' | 'variables' | 'users') => {
    const validation = validateFile(file, type);

    setFiles(prev => ({
      ...prev,
      [type]: {
        file,
        status: validation.valid ? 'valid' : 'invalid',
        error: validation.error
      }
    }));

    // Reset validation result when files change
    setValidationResult(null);
  };

  const handleDrop = (e: React.DragEvent, type: 'scenario' | 'variables' | 'users') => {
    e.preventDefault();
    const droppedFile = e.dataTransfer.files[0];
    if (droppedFile) {
      handleFileUpload(droppedFile, type);
    }
  };

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>, type: 'scenario' | 'variables' | 'users') => {
    const selectedFile = e.target.files?.[0];
    if (selectedFile) {
      handleFileUpload(selectedFile, type);
    }
  };

  const handleSubmit = async () => {
    if (!files.scenario.file || !files.variables.file) {
      return;
    }

    setIsSubmitting(true);
    setValidationResult(null);

    try {
      const formData = new FormData();
      formData.append('scenario', files.scenario.file);
      formData.append('variables', files.variables.file);

      if (files.users.file) {
        formData.append('users', files.users.file);
      }

      const response = await fetch(`${window.location.protocol}//${window.location.hostname}:8080/upload/validate`, {
        method: 'POST',
        body: formData,
      });

      const result: ValidationResponse = await response.json();
      setValidationResult(result);

    } catch (error) {
      setValidationResult({
        status: 'error',
        message: 'Erreur de connexion à l\'API',
        errors: ['Impossible de contacter le serveur'],
        warnings: [],
        analysis: {
          variables_found: { user: [], env: [], extract: [] },
          csv_columns: [],
          files_saved: false
        }
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleExecuteTest = async () => {
    setIsSubmitting(true);

    try {
      const response = await fetch(`${window.location.protocol}//${window.location.hostname}:8080/execute`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
      });

      const result = await response.json();

      if (response.ok) {
        // Succès - afficher le résultat
        alert(`Test lancé avec succès!\nTest ID: ${result.test_id}\nStatut: ${result.status}\nMessage: ${result.message}`);

        // Reset de l'interface après succès
        setValidationResult(null);
        setFiles({
          scenario: { file: null, status: 'idle' },
          variables: { file: null, status: 'idle' },
          users: { file: null, status: 'idle' }
        });
      } else {
        // Erreur
        alert(`Erreur lors du lancement du test:\n${result.error || result.message}`);
      }

    } catch (error) {
      alert(`Erreur de connexion:\n${error}`);
    } finally {
      setIsSubmitting(false);
    }
  };

  const getFileStatusColor = (status: FileState['status']) => {
    switch (status) {
      case 'valid': return 'border-green-500 bg-green-50';
      case 'invalid': return 'border-red-500 bg-red-50';
      default: return 'border-gray-300 hover:border-blue-400';
    }
  };

  const getFileStatusIcon = (status: FileState['status']) => {
    switch (status) {
      case 'valid': return '✅';
      case 'invalid': return '❌';
      default: return '📄';
    }
  };

  const allFilesValid = files.scenario.status === 'valid' && files.variables.status === 'valid';

  return (
    <div className="min-h-screen bg-gray-50 py-8">
      <div className="max-w-4xl mx-auto px-4">
        {/* Header */}
        <div className="text-center mb-8">
          <h1 className="text-3xl font-bold text-gray-900 mb-2">
            API Load & Stress Test
          </h1>
          <p className="text-gray-600">
            Uploadez vos fichiers de configuration pour démarrer un test
          </p>
        </div>

        {/* Upload zones */}
        <div className="grid gap-6 md:grid-cols-3 mb-8">
          {/* Scenario.json */}
          <div
            className={`relative border-2 border-dashed rounded-lg p-6 text-center transition-colors ${getFileStatusColor(files.scenario.status)}`}
            onDrop={(e) => handleDrop(e, 'scenario')}
            onDragOver={(e) => e.preventDefault()}
          >
            <input
              type="file"
              accept=".json"
              onChange={(e) => handleFileSelect(e, 'scenario')}
              className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
            />
            <div className="text-4xl mb-2">{getFileStatusIcon(files.scenario.status)}</div>
            <h3 className="font-semibold text-gray-900 mb-1">scenario.json</h3>
            <p className="text-sm text-gray-500 mb-2">Description des étapes du test</p>
            {files.scenario.file ? (
              <p className="text-xs text-gray-700 font-medium">{files.scenario.file.name}</p>
            ) : (
              <p className="text-xs text-gray-400">Glissez ou cliquez pour uploader</p>
            )}
            {files.scenario.error && (
              <p className="text-xs text-red-600 mt-1">{files.scenario.error}</p>
            )}
          </div>

          {/* Variables.json */}
          <div
            className={`relative border-2 border-dashed rounded-lg p-6 text-center transition-colors ${getFileStatusColor(files.variables.status)}`}
            onDrop={(e) => handleDrop(e, 'variables')}
            onDragOver={(e) => e.preventDefault()}
          >
            <input
              type="file"
              accept=".json"
              onChange={(e) => handleFileSelect(e, 'variables')}
              className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
            />
            <div className="text-4xl mb-2">{getFileStatusIcon(files.variables.status)}</div>
            <h3 className="font-semibold text-gray-900 mb-1">variables.json</h3>
            <p className="text-sm text-gray-500 mb-2">Paramètres d'exécution</p>
            {files.variables.file ? (
              <p className="text-xs text-gray-700 font-medium">{files.variables.file.name}</p>
            ) : (
              <p className="text-xs text-gray-400">Glissez ou cliquez pour uploader</p>
            )}
            {files.variables.error && (
              <p className="text-xs text-red-600 mt-1">{files.variables.error}</p>
            )}
          </div>

          {/* Users.csv - OPTIONNEL */}
          <div
            className={`relative border-2 border-dashed rounded-lg p-6 text-center transition-colors ${getFileStatusColor(files.users.status)}`}
            onDrop={(e) => handleDrop(e, 'users')}
            onDragOver={(e) => e.preventDefault()}
          >
            <input
              type="file"
              accept=".csv"
              onChange={(e) => handleFileSelect(e, 'users')}
              className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
            />
            <div className="text-4xl mb-2">{getFileStatusIcon(files.users.status)}</div>
            <h3 className="font-semibold text-gray-900 mb-1">
              users.csv <span className="text-sm text-blue-600 font-normal">(optionnel)</span>
            </h3>
            <p className="text-sm text-gray-500 mb-2">Données utilisateurs pour authentification</p>
            {files.users.file ? (
              <p className="text-xs text-gray-700 font-medium">{files.users.file.name}</p>
            ) : (
              <p className="text-xs text-gray-400">Glissez ou cliquez pour uploader</p>
            )}
            {files.users.error && (
              <p className="text-xs text-red-600 mt-1">{files.users.error}</p>
            )}
          </div>
        </div>

        {/* Status summary */}
        <div className="bg-white rounded-lg shadow-sm border p-6 mb-6">
          <h3 className="font-semibold text-gray-900 mb-3">État des fichiers</h3>
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-sm text-gray-600">Scenario JSON</span>
              <span className={`text-sm font-medium ${files.scenario.status === 'valid' ? 'text-green-600' : files.scenario.status === 'invalid' ? 'text-red-600' : 'text-gray-400'}`}>
                {files.scenario.status === 'valid' ? 'Valide' : files.scenario.status === 'invalid' ? 'Invalide' : 'En attente'}
              </span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm text-gray-600">Variables JSON</span>
              <span className={`text-sm font-medium ${files.variables.status === 'valid' ? 'text-green-600' : files.variables.status === 'invalid' ? 'text-red-600' : 'text-gray-400'}`}>
                {files.variables.status === 'valid' ? 'Valide' : files.variables.status === 'invalid' ? 'Invalide' : 'En attente'}
              </span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm text-gray-600">Users CSV <span className="text-blue-600">(optionnel)</span></span>
              <span className={`text-sm font-medium ${files.users.status === 'valid' ? 'text-green-600' : files.users.status === 'invalid' ? 'text-red-600' : 'text-gray-400'}`}>
                {files.users.status === 'valid' ? 'Fourni' : files.users.status === 'invalid' ? 'Invalide' : 'Non fourni'}
              </span>
            </div>
          </div>
        </div>

        {/* Validation Results */}
        {validationResult && (
          <div className={`rounded-lg border p-6 mb-6 ${validationResult.status === 'success' ? 'bg-green-50 border-green-200' : 'bg-red-50 border-red-200'
            }`}>
            <div className="flex items-center mb-4">
              <span className="text-2xl mr-3">
                {validationResult.status === 'success' ? '✅' : '❌'}
              </span>
              <div>
                <h3 className={`font-semibold ${validationResult.status === 'success' ? 'text-green-800' : 'text-red-800'
                  }`}>
                  {validationResult.message}
                </h3>
              </div>
            </div>

            {/* Errors */}
            {validationResult.errors.length > 0 && (
              <div className="mb-4">
                <h4 className="font-medium text-red-800 mb-2">Erreurs détectées :</h4>
                <ul className="list-disc list-inside space-y-1">
                  {validationResult.errors.map((error, index) => (
                    <li key={index} className="text-sm text-red-700">{error}</li>
                  ))}
                </ul>
              </div>
            )}

            {/* Warnings */}
            {validationResult.warnings.length > 0 && (
              <div className="mb-4">
                <h4 className="font-medium text-yellow-800 mb-2">Avertissements :</h4>
                <ul className="list-disc list-inside space-y-1">
                  {validationResult.warnings.map((warning, index) => (
                    <li key={index} className="text-sm text-yellow-700">{warning}</li>
                  ))}
                </ul>
              </div>
            )}

            {/* Analysis - Success only */}
            {validationResult.status === 'success' && (
              <div className="mt-4">
                <h4 className="font-medium text-green-800 mb-3">Analyse intelligente :</h4>
                <div className="grid gap-4 md:grid-cols-2">
                  {/* Variables utilisateur */}
                  {validationResult.analysis.variables_found.user.length > 0 && (
                    <div className="bg-white rounded-lg p-4 border border-green-200">
                      <h5 className="font-medium text-gray-900 mb-2">Variables utilisateur détectées :</h5>
                      <div className="space-y-1">
                        {validationResult.analysis.variables_found.user.map((userVar, index) => (
                          <span key={index} className="inline-block bg-blue-100 text-blue-800 text-xs font-medium px-2 py-1 rounded mr-2">
                            user.{userVar}
                          </span>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Variables environnement */}
                  {validationResult.analysis.variables_found.env.length > 0 && (
                    <div className="bg-white rounded-lg p-4 border border-green-200">
                      <h5 className="font-medium text-gray-900 mb-2">Variables environnement :</h5>
                      <div className="space-y-1">
                        {validationResult.analysis.variables_found.env.map((envVar, index) => (
                          <span key={index} className="inline-block bg-green-100 text-green-800 text-xs font-medium px-2 py-1 rounded mr-2">
                            env.{envVar}
                          </span>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Variables extraites */}
                  {validationResult.analysis.variables_found.extract.length > 0 && (
                    <div className="bg-white rounded-lg p-4 border border-green-200">
                      <h5 className="font-medium text-gray-900 mb-2">Variables extraites :</h5>
                      <div className="space-y-1">
                        {validationResult.analysis.variables_found.extract.map((extractVar, index) => (
                          <span key={index} className="inline-block bg-purple-100 text-purple-800 text-xs font-medium px-2 py-1 rounded mr-2">
                            {extractVar}
                          </span>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Colonnes CSV */}
                  {validationResult.analysis.csv_columns.length > 0 && (
                    <div className="bg-white rounded-lg p-4 border border-green-200">
                      <h5 className="font-medium text-gray-900 mb-2">Colonnes CSV disponibles :</h5>
                      <div className="space-y-1">
                        {validationResult.analysis.csv_columns.map((column, index) => (
                          <span key={index} className="inline-block bg-gray-100 text-gray-800 text-xs font-medium px-2 py-1 rounded mr-2">
                            {column}
                          </span>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        )}

        {/* Action buttons */}
        <div className="text-center space-y-4">
          {!validationResult && (
            <button
              onClick={handleSubmit}
              disabled={!allFilesValid || isSubmitting}
              className={`px-8 py-3 rounded-lg font-medium transition-colors ${allFilesValid && !isSubmitting
                ? 'bg-blue-600 text-white hover:bg-blue-700'
                : 'bg-gray-300 text-gray-500 cursor-not-allowed'
                }`}
            >
              {isSubmitting ? 'Validation en cours...' :
                allFilesValid ? 'Valider et analyser' : 'Uploadez scenario.json et variables.json pour continuer'}
            </button>
          )}

          {validationResult?.status === 'success' && (
            <div className="space-x-4">
              <button
                onClick={() => {
                  setValidationResult(null);
                  setFiles({
                    scenario: { file: null, status: 'idle' },
                    variables: { file: null, status: 'idle' },
                    users: { file: null, status: 'idle' }
                  });
                }}
                className="px-6 py-2 border border-gray-300 rounded-lg text-gray-700 hover:bg-gray-50 transition-colors"
              >
                Modifier les fichiers
              </button>
              <button
                onClick={handleExecuteTest}
                disabled={isSubmitting}
                className={`px-8 py-3 rounded-lg font-medium transition-colors ${isSubmitting
                  ? 'bg-gray-400 text-gray-600 cursor-not-allowed'
                  : 'bg-green-600 text-white hover:bg-green-700'
                  }`}
              >
                {isSubmitting ? 'Exécution en cours...' : 'Lancer le test de charge'}
              </button>
            </div>
          )}

          {validationResult?.status === 'error' && (
            <button
              onClick={() => setValidationResult(null)}
              className="px-6 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors"
            >
              Corriger et réessayer
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
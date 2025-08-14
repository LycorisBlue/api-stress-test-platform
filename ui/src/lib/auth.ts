// Interfaces
interface LoginCredentials {
 username: string;
 password: string;
}

interface AuthResponse {
 access_token: string;
 token_type: string;
 expires_in: number;
 username: string;
}

interface AuthError {
 detail: string;
}

// Configuration
const API_BASE_URL = `${typeof window !== 'undefined' ? window.location.protocol : 'http:'}//${typeof window !== 'undefined' ? window.location.hostname : 'localhost'}:8080`;
const TOKEN_KEY = 'auth_token';
const USERNAME_KEY = 'username';

// Gestion du token
export const setToken = (token: string): void => {
 if (typeof window !== 'undefined') {
   localStorage.setItem(TOKEN_KEY, token);
 }
};

export const getToken = (): string | null => {
 if (typeof window !== 'undefined') {
   return localStorage.getItem(TOKEN_KEY);
 }
 return null;
};

export const removeToken = (): void => {
 if (typeof window !== 'undefined') {
   localStorage.removeItem(TOKEN_KEY);
   localStorage.removeItem(USERNAME_KEY);
 }
};

// Gestion du username
export const setUsername = (username: string): void => {
 if (typeof window !== 'undefined') {
   localStorage.setItem(USERNAME_KEY, username);
 }
};

export const getUsername = (): string | null => {
 if (typeof window !== 'undefined') {
   return localStorage.getItem(USERNAME_KEY);
 }
 return null;
};

// Vérification de l'authentification
export const isAuthenticated = (): boolean => {
 return getToken() !== null;
};

// Connexion
export const login = async (credentials: LoginCredentials): Promise<AuthResponse> => {
 const response = await fetch(`${API_BASE_URL}/auth/login`, {
   method: 'POST',
   headers: {
     'Content-Type': 'application/json',
   },
   body: JSON.stringify(credentials),
 });

 if (!response.ok) {
   const error: AuthError = await response.json();
   throw new Error(error.detail || 'Erreur de connexion');
 }

 const authData: AuthResponse = await response.json();
 
 // Stocker le token et le username
 setToken(authData.access_token);
 setUsername(authData.username);
 
 return authData;
};

// Déconnexion
export const logout = async (): Promise<void> => {
 const token = getToken();
 
 if (token) {
   try {
     await fetch(`${API_BASE_URL}/auth/logout`, {
       method: 'POST',
       headers: {
         'Authorization': `Bearer ${token}`,
         'Content-Type': 'application/json',
       },
     });
   } catch (error) {
     console.warn('Erreur lors de la déconnexion côté serveur:', error);
   }
 }
 
 // Nettoyer le stockage local dans tous les cas
 removeToken();
};

// Vérification du token côté serveur
export const verifyToken = async (): Promise<boolean> => {
 const token = getToken();
 
 if (!token) {
   return false;
 }

 try {
   const response = await fetch(`${API_BASE_URL}/auth/verify`, {
     method: 'GET',
     headers: {
       'Authorization': `Bearer ${token}`,
     },
   });

   if (!response.ok) {
     removeToken();
     return false;
   }

   return true;
 } catch (error) {
   console.error('Erreur de vérification du token:', error);
   removeToken();
   return false;
 }
};

// Helper pour les appels API authentifiés
export const authenticatedFetch = async (
 url: string,
 options: RequestInit = {}
): Promise<Response> => {
 const token = getToken();
 
 if (!token) {
   throw new Error('Token manquant');
 }

 const headers = {
   'Authorization': `Bearer ${token}`,
   ...options.headers,
 };

 const response = await fetch(url, {
   ...options,
   headers,
 });

 // Si le token est invalide, nettoyer et rediriger
 if (response.status === 401) {
   removeToken();
   if (typeof window !== 'undefined') {
     window.location.href = '/login';
   }
   throw new Error('Session expirée');
 }

 return response;
};

// Helper pour construire l'URL de l'API
export const getApiUrl = (endpoint: string): string => {
 return `${API_BASE_URL}${endpoint}`;
};
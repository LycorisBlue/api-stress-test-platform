import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';

export function middleware(request: NextRequest) {
    // Pages publiques (non protégées)
    const publicPaths = ['/login'];

    // Vérifier si la route actuelle est publique
    const isPublicPath = publicPaths.some(path =>
        request.nextUrl.pathname.startsWith(path)
    );

    // Récupérer le token depuis les cookies ou headers
    const token = request.cookies.get('auth_token')?.value ||
        request.headers.get('authorization')?.replace('Bearer ', '');

    // Si pas de token et route protégée → rediriger vers login
    if (!token && !isPublicPath) {
        const loginUrl = new URL('/login', request.url);
        return NextResponse.redirect(loginUrl);
    }

    // Si token présent et sur page login → rediriger vers accueil
    if (token && request.nextUrl.pathname === '/login') {
        const homeUrl = new URL('/', request.url);
        return NextResponse.redirect(homeUrl);
    }

    // Continuer normalement
    return NextResponse.next();
}

// Configuration des routes à protéger
export const config = {
    matcher: [
        /*
         * Protéger toutes les routes sauf :
         * - api routes (gérées côté serveur)
         * - _next/static (fichiers statiques)
         * - _next/image (optimisation d'images)
         * - favicon.ico
         */
        '/((?!api|_next/static|_next/image|favicon.ico).*)',
    ],
};
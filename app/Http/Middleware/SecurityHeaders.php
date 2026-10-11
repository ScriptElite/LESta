<?php

namespace App\Http\Middleware;

use Closure;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Vite;
use Symfony\Component\HttpFoundation\Response;

/**
 * Browser-side defences for every page of the panel: a content security policy that only runs
 * scripts from this site and the one inline script that carries this response's nonce (so injected
 * markup cannot run script), no framing (clickjacking), no content-type guessing, a referrer that
 * stays on the site, and HSTS once the page is served over HTTPS.
 *
 * Styles allow inline: the interface sets style attributes (progress bars, positions) and Tailwind
 * is built to files. form-action is deliberately not set: some actions redirect to a node's webmail
 * or database tool, a different site, and browsers apply form-action to redirects.
 */
class SecurityHeaders
{
    /**
     * @param  Closure(Request): (Response)  $next
     */
    public function handle(Request $request, Closure $next): Response
    {
        // Under the Vite dev server the page loads scripts from another origin and injects its own
        // client, which a strict policy would block; only the other headers apply there.
        $nonce = Vite::isRunningHot() ? null : Vite::useCspNonce();

        $response = $next($request);

        $response->headers->set('X-Content-Type-Options', 'nosniff');
        $response->headers->set('X-Frame-Options', 'DENY');
        $response->headers->set('Referrer-Policy', 'same-origin');
        $response->headers->set('Permissions-Policy', 'camera=(), microphone=(), geolocation=(), payment=()');

        if ($nonce !== null) {
            $response->headers->set('Content-Security-Policy', implode('; ', [
                "default-src 'self'",
                "script-src 'self' 'nonce-{$nonce}'",
                "style-src 'self' 'unsafe-inline'",
                "img-src 'self' data: blob:",
                "font-src 'self' data:",
                "connect-src 'self'",
                "object-src 'none'",
                "base-uri 'self'",
                "frame-ancestors 'none'",
            ]));
        }

        if ($request->isSecure()) {
            $response->headers->set('Strict-Transport-Security', 'max-age=31536000');
        }

        return $response;
    }
}

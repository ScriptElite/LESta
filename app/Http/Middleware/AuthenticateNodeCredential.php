<?php

namespace App\Http\Middleware;

use App\Enums\NodeEnrollmentStatus;
use App\Models\Node;
use Closure;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\RateLimiter;
use Symfony\Component\HttpFoundation\Response;

class AuthenticateNodeCredential
{
    /** Failed credentials one address may present per minute before it is refused outright. */
    private const int MAX_FAILURES_PER_MINUTE = 30;

    /**
     * Handle an incoming request.
     *
     * Authenticates a bearer node credential against nodes.node_credential_hash and stores the
     * resolved, enrolled Node on the request's attribute bag under "node" for the controller to
     * read. This is machine-to-machine, bearer-token authentication over Laravel's own
     * already-terminated HTTPS, never a session or a Sanctum guard.
     *
     * A suspended node's credential is rejected here too, not just hidden in the UI: an admin
     * suspending a node is expected to actually cut it off, not merely stop showing it as active
     * while it keeps heartbeating and submitting operation results underneath.
     *
     * @param  Closure(Request): (Response)  $next
     */
    public function handle(Request $request, Closure $next): Response
    {
        $token = $request->bearerToken();

        // Failed attempts are counted per address before anything is looked up: the 'agent'
        // limiter that follows this middleware is keyed by the presented token, so a caller
        // sending a new random token each time would never reach it.
        $failureKey = 'agent-auth-failures:'.$request->ip();

        if (RateLimiter::tooManyAttempts($failureKey, self::MAX_FAILURES_PER_MINUTE)) {
            abort(429, 'Too many failed attempts.');
        }

        if ($token === null) {
            RateLimiter::hit($failureKey, 60);

            abort(401, 'Missing bearer token.');
        }

        $node = Node::query()
            ->where('node_credential_hash', hash('sha256', $token))
            ->where('enrollment_status', NodeEnrollmentStatus::Enrolled->value)
            ->whereNull('suspended_at')
            ->first();

        if ($node === null) {
            RateLimiter::hit($failureKey, 60);

            abort(401, 'Invalid, unenrolled, or suspended node credential.');
        }

        $request->attributes->set('node', $node);

        return $next($request);
    }
}

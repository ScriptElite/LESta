<?php

test('every page carries the browser security headers', function () {
    $response = $this->get(route('login'));

    $response->assertOk()
        ->assertHeader('X-Content-Type-Options', 'nosniff')
        ->assertHeader('X-Frame-Options', 'DENY')
        ->assertHeader('Referrer-Policy', 'same-origin');

    expect($response->headers->get('Permissions-Policy'))->toContain('camera=()');
});

test('the content security policy only allows this site and the nonce of the inline script', function () {
    $response = $this->get(route('login'));
    $policy = $response->headers->get('Content-Security-Policy');

    expect($policy)->toContain("default-src 'self'")
        ->and($policy)->toContain("object-src 'none'")
        ->and($policy)->toContain("frame-ancestors 'none'")
        ->and($policy)->toContain("base-uri 'self'")
        ->and($policy)->not->toContain('unsafe-eval')
        ->and($policy)->not->toContain("script-src 'self' 'unsafe-inline'");

    preg_match("/script-src 'self' 'nonce-([A-Za-z0-9+\/=_-]+)'/", $policy, $matches);

    expect($matches)->toHaveCount(2);

    // The appearance script and the build's script tags carry exactly that nonce.
    $response->assertSee('<script nonce="'.$matches[1].'">', false);
});

test('each response gets its own nonce', function () {
    $nonce = fn () => preg_match("/'nonce-([^']+)'/", $this->get(route('login'))->headers->get('Content-Security-Policy'), $m) ? $m[1] : null;

    expect($nonce())->not->toBe($nonce());
});

test('strict transport security is sent only over https', function () {
    $this->get(route('login'))->assertHeaderMissing('Strict-Transport-Security');

    // Behind the TLS-terminating proxy the app trusts, the forwarded protocol marks it secure.
    $this->get(route('login'), ['X-Forwarded-Proto' => 'https'])->assertHeader('Strict-Transport-Security', 'max-age=31536000');
});

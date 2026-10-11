<?php

use App\Models\Node;
use Illuminate\Support\Facades\RateLimiter;

beforeEach(function () {
    RateLimiter::clear('agent-auth-failures:127.0.0.1');
});

test('an address presenting many wrong node credentials is refused before any lookup', function () {
    for ($i = 0; $i < 30; $i++) {
        $this->withToken(bin2hex(random_bytes(40)))->postJson('/agent/v1/heartbeat', [])->assertUnauthorized();
    }

    $this->withToken(bin2hex(random_bytes(40)))->postJson('/agent/v1/heartbeat', [])->assertStatus(429);
    $this->postJson('/agent/v1/heartbeat', [])->assertStatus(429);
});

test('wrong credentials from one address do not affect another address', function () {
    for ($i = 0; $i < 31; $i++) {
        $this->withToken(bin2hex(random_bytes(40)))->postJson('/agent/v1/heartbeat', []);
    }

    $this->withToken(bin2hex(random_bytes(40)))
        ->withServerVariables(['REMOTE_ADDR' => '203.0.113.7'])
        ->postJson('/agent/v1/heartbeat', [])
        ->assertUnauthorized();
});

test('a node with a valid credential is not counted as a failure', function () {
    $credential = Node::factory()->create()->completeEnrollment('1', '1.0.0');

    for ($i = 0; $i < 40; $i++) {
        $this->withToken($credential)->postJson('/agent/v1/heartbeat', [])->assertStatus(422);
    }

    expect(RateLimiter::attempts('agent-auth-failures:127.0.0.1'))->toBe(0);
});

test('forwarded client addresses are believed only from the proxies named for it', function () {
    $this->get(route('login'), ['X-Forwarded-For' => '198.51.100.9']);
    expect(request()->ip())->toBe('198.51.100.9');

    // A machine on the private network that is not the proxy cannot claim another address.
    $this->withServerVariables(['REMOTE_ADDR' => '10.1.2.3'])->get(route('login'), ['X-Forwarded-For' => '198.51.100.9']);
    expect(request()->ip())->toBe('10.1.2.3');
});

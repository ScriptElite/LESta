<?php

namespace App\Http\Controllers\Backups;

use App\Concerns\ResolvesCurrentAccount;
use App\Http\Controllers\Controller;
use App\Models\AccountBackup;
use App\Models\AccountBackupDestination;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Gate;
use Inertia\Inertia;

/**
 * The account's own S3-compatible storage that backups are copied to.
 */
class AccountBackupDestinationController extends Controller
{
    use ResolvesCurrentAccount;

    /**
     * Saves the account's own S3-compatible storage. The keys are write-only: left empty on a later
     * save they keep their saved values.
     */
    public function update(Request $request): RedirectResponse
    {
        $account = $this->resolveAccount($request->user());

        abort_if($account === null, 404);

        Gate::authorize('create', [AccountBackup::class, $account]);

        $existing = $account->backupDestination;

        $data = $request->validate([
            'enabled' => ['nullable', 'boolean'],
            'endpoint' => [
                'required', 'string', 'max:255',
                function (string $attribute, mixed $value, \Closure $fail): void {
                    $parts = parse_url((string) $value);

                    if ($parts === false || ($parts['scheme'] ?? '') !== 'https' || ($parts['host'] ?? '') === '' || isset($parts['user']) || isset($parts['query']) || isset($parts['fragment']) || ! in_array($parts['path'] ?? '', ['', '/'], true)) {
                        $fail(__('Use an https address of the storage service, such as https://s3.eu-west-1.amazonaws.com.'));
                    }
                },
            ],
            'region' => ['required', 'string', 'regex:'.AccountBackupDestination::REGION_PATTERN],
            'bucket' => ['required', 'string', 'regex:'.AccountBackupDestination::BUCKET_PATTERN],
            'prefix' => ['nullable', 'string', 'regex:'.AccountBackupDestination::PREFIX_PATTERN, fn (string $a, mixed $v, \Closure $fail) => str_contains((string) $v, '..') || str_contains((string) $v, '//') ? $fail(__('The folder cannot contain .. or //.')) : null],
            'access_key' => [$existing === null ? 'required' : 'nullable', 'string', 'regex:'.AccountBackupDestination::KEY_PATTERN],
            'secret_key' => [$existing === null ? 'required' : 'nullable', 'string', 'regex:'.AccountBackupDestination::SECRET_PATTERN],
        ], [
            'region.regex' => __('The region is lower-case letters, numbers and dashes, such as eu-west-1.'),
            'bucket.regex' => __('The bucket name is not valid.'),
            'prefix.regex' => __('The folder can use letters, numbers and . _ - / only.'),
            'access_key.regex' => __('The access key is not valid.'),
            'secret_key.regex' => __('The secret key is not valid.'),
        ]);

        $attributes = [
            'enabled' => (bool) ($data['enabled'] ?? false),
            'endpoint' => rtrim($data['endpoint'], '/'),
            'region' => $data['region'],
            'bucket' => $data['bucket'],
            'prefix' => $data['prefix'] ?? '',
        ];

        if (($data['access_key'] ?? '') !== '') {
            $attributes['access_key'] = $data['access_key'];
        }

        if (($data['secret_key'] ?? '') !== '') {
            $attributes['secret_key'] = $data['secret_key'];
        }

        if ($existing === null) {
            $account->backupDestination()->create($attributes);
        } else {
            $existing->update($attributes);
        }

        Inertia::flash('toast', ['type' => 'success', 'message' => __('Your storage is saved.')]);

        return to_route('account-backups.index');
    }

    public function destroy(Request $request): RedirectResponse
    {
        $account = $this->resolveAccount($request->user());

        abort_if($account === null, 404);

        Gate::authorize('create', [AccountBackup::class, $account]);

        $account->backupDestination()->delete();

        Inertia::flash('toast', ['type' => 'success', 'message' => __('Your storage is removed. Copies already made stay where they are.')]);

        return to_route('account-backups.index');
    }
}

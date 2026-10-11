<?php

namespace App\Http\Controllers\Backups;

use App\Concerns\ResolvesCurrentAccount;
use App\Http\Controllers\Controller;
use App\Models\AccountBackup;
use App\Models\AccountBackupSchedule;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Gate;
use Illuminate\Validation\Rule;
use Inertia\Inertia;

/**
 * An account's automatic backup schedule: off, daily or weekly, and what to include.
 */
class AccountBackupScheduleController extends Controller
{
    use ResolvesCurrentAccount;

    /**
     * Sets the account's automatic backup schedule: off, daily or weekly, and what to include.
     */
    public function update(Request $request): RedirectResponse
    {
        $account = $this->resolveAccount($request->user());

        abort_if($account === null, 404);

        Gate::authorize('create', [AccountBackup::class, $account]);

        $data = $request->validate([
            'frequency' => ['required', Rule::in(AccountBackupSchedule::FREQUENCIES)],
            'parts' => ['required_unless:frequency,off', 'array'],
            'parts.*' => ['string', Rule::in(AccountBackup::PARTS)],
        ], ['parts.required_unless' => __('Choose what to back up.')]);

        $schedule = $account->backupSchedule ?? new AccountBackupSchedule(['account_id' => $account->id, 'parts' => AccountBackup::PARTS]);

        $parts = array_values(array_intersect(AccountBackup::PARTS, $data['parts'] ?? $schedule->parts));

        $schedule->forceFill(['account_id' => $account->id, 'frequency' => $data['frequency'], 'parts' => $parts === [] ? AccountBackup::PARTS : $parts]);
        $schedule->forceFill(['next_run_at' => $data['frequency'] === 'off' ? null : $schedule->nextRunAfter(now())])->save();

        Inertia::flash('toast', ['type' => 'success', 'message' => $data['frequency'] === 'off' ? __('Automatic backups are off.') : __('Automatic backups saved.')]);

        return to_route('account-backups.index');
    }
}

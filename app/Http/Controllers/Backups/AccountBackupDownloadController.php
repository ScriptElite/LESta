<?php

namespace App\Http\Controllers\Backups;

use App\Actions\AccountBackups\PrepareAccountBackupDownload;
use App\Exceptions\NoBackupCapableNodeAvailableException;
use App\Http\Controllers\Controller;
use App\Models\AccountBackup;
use App\Models\AccountBackupDownload;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Gate;
use Illuminate\Support\Facades\Storage;
use Illuminate\Support\Str;
use Illuminate\Validation\ValidationException;
use Inertia\Inertia;
use Symfony\Component\HttpFoundation\StreamedResponse;

/**
 * Plain tar.gz copies of a backup for the owner to download (see PrepareAccountBackupDownload).
 */
class AccountBackupDownloadController extends Controller
{
    /**
     * Asks the node to prepare a downloadable copy; the page shows it when it is ready.
     */
    public function store(Request $request, AccountBackup $backup): RedirectResponse
    {
        try {
            app(PrepareAccountBackupDownload::class)->handle($request->user(), $backup);
        } catch (NoBackupCapableNodeAvailableException) {
            throw ValidationException::withMessages(['backup' => __('Backups are not available on :node.', ['node' => $backup->node->name])]);
        }

        Inertia::flash('toast', ['type' => 'success', 'message' => __('Preparing the download. It appears here in a moment.')]);

        return to_route('account-backups.index');
    }

    /**
     * Streams a ready copy as a tar.gz. It stays available until it expires.
     */
    public function show(AccountBackup $backup, AccountBackupDownload $download): StreamedResponse
    {
        Gate::authorize('download', $backup);

        abort_unless($download->account_backup_id === $backup->id && $download->isReady() && $download->path !== null && Storage::disk('local')->exists($download->path), 404);

        $name = Str::slug($backup->account->name).'-backup-'.($backup->completed_at ?? $backup->created_at)?->format('Y-m-d-Hi').'.tar.gz';

        return Storage::disk('local')->download($download->path, $name, ['Content-Type' => 'application/gzip']);
    }
}

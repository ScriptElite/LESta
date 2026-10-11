<?php

use App\Http\Controllers\Backups\AccountBackupController;
use App\Http\Controllers\Backups\AccountBackupDestinationController;
use App\Http\Controllers\Backups\AccountBackupDownloadController;
use App\Http\Controllers\Backups\AccountBackupImportController;
use App\Http\Controllers\Backups\AccountBackupScheduleController;
use Illuminate\Support\Facades\Route;

Route::middleware(['auth', 'verified'])->group(function () {
    Route::get('account-backups', [AccountBackupController::class, 'index'])->name('account-backups.index');
    Route::post('account-backups', [AccountBackupController::class, 'store'])->name('account-backups.store');
    Route::put('account-backups/schedule', [AccountBackupScheduleController::class, 'update'])->name('account-backups.schedule');
    Route::put('account-backups/destination', [AccountBackupDestinationController::class, 'update'])->name('account-backups.destination');
    Route::delete('account-backups/destination', [AccountBackupDestinationController::class, 'destroy'])->name('account-backups.destination.destroy');
    Route::post('account-backups/import/storage', [AccountBackupImportController::class, 'fromStorage'])->name('account-backups.import.storage');
    Route::post('account-backups/import/uploads', [AccountBackupImportController::class, 'begin'])->name('account-backups.import.begin');
    Route::put('account-backups/import/uploads/{import}/chunk', [AccountBackupImportController::class, 'chunk'])->middleware('throttle:1200,1')->name('account-backups.import.chunk');
    Route::post('account-backups/{backup}/copy', [AccountBackupController::class, 'copy'])->name('account-backups.copy');
    Route::post('account-backups/{backup}/restore', [AccountBackupController::class, 'restore'])->name('account-backups.restore');
    Route::post('account-backups/{backup}/download', [AccountBackupDownloadController::class, 'store'])->name('account-backups.prepare-download');
    Route::get('account-backups/{backup}/downloads/{download}', [AccountBackupDownloadController::class, 'show'])->name('account-backups.download');
    Route::delete('account-backups/{backup}', [AccountBackupController::class, 'destroy'])->name('account-backups.destroy');
});

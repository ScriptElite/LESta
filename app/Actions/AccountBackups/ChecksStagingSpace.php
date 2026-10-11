<?php

namespace App\Actions\AccountBackups;

use Illuminate\Support\Facades\Storage;

/**
 * Whether the control plane's disk has room to hold a backup file for a while: an uploaded backup
 * waiting for its node, or a prepared download. Both are kept under storage/ and are as large as
 * 4 GB, and any account owner can start one, so a start is refused when the disk could not take it
 * with a margin to spare.
 */
class ChecksStagingSpace
{
    /** Space that must remain free after the file, so the panel's own logs and sessions keep working. */
    private const int MARGIN_BYTES = 2 * 1024 * 1024 * 1024;

    public function hasRoomFor(int $bytes): bool
    {
        $free = @disk_free_space(Storage::disk('local')->path(''));

        return $free === false || $free >= $bytes + self::MARGIN_BYTES;
    }
}

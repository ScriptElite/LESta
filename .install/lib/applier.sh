# shellcheck shell=sh
# shellcheck disable=SC3043
#
# .install/lib/applier.sh
#
# The root applier: the one process that renders and applies node configuration for the
# unprivileged lesta-agent-daemon (see agent/internal/applier and the vault's "Privileged Applier
# Split Plan"). A leaf installer whose capability has moved behind the applier calls, in order:
#
#   applier_install_unit                      once per node; idempotent
#   applier_allow_paths <capability> <path>…  the directories that capability writes
#   applier_allow_capabilities <capability> <CAP_…>…  any extra Linux capabilities it needs
#   applier_start                             enable and (re)start the unit
#   applier_enable_capability <capability>    the switch: the daemon now hands it to the applier
#
# and, after swapping in a new agent binary, applier_restart_if_enabled before any self-test, so
# the self-test runs against the new applier.
#
# The unit is sandboxed: it can write only the paths the capabilities that moved behind it name
# (one drop-in each), has no network beyond what the tools it runs need, and keeps only the
# capabilities it uses.

APPLIER_UNIT_PATH="/etc/systemd/system/lesta-applier.service"
APPLIER_DROPIN_DIR="/etc/systemd/system/lesta-applier.service.d"
APPLIER_CAPABILITIES_PATH="/etc/lesta/applier-capabilities"

applier_install_unit() {
    cat > "${APPLIER_UNIT_PATH}.tmp" <<'UNIT'
[Unit]
Description=LESta privileged applier (applies node configuration for the agent daemon)
After=local-fs.target
Before=lesta-agent-daemon.service

[Service]
Type=simple
User=root
ExecStart=/var/lib/lesta/agent/bin/lesta-agent applier
Restart=on-failure
RestartSec=2
RuntimeDirectory=lesta
RuntimeDirectoryMode=0755
UMask=0022

# Sandbox. ReadWritePaths is extended per capability by a drop-in (applier_allow_paths).
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
PrivateDevices=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectKernelLogs=yes
ProtectControlGroups=yes
ProtectClock=yes
RestrictSUIDSGID=yes
RestrictRealtime=yes
RestrictNamespaces=yes
LockPersonality=yes
SystemCallArchitectures=native
SystemCallFilter=@system-service
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
CapabilityBoundingSet=CAP_CHOWN CAP_FOWNER CAP_DAC_OVERRIDE CAP_DAC_READ_SEARCH CAP_KILL

[Install]
WantedBy=multi-user.target
UNIT
    chmod 0644 "${APPLIER_UNIT_PATH}.tmp"
    chown root:root "${APPLIER_UNIT_PATH}.tmp"
    mv -f "${APPLIER_UNIT_PATH}.tmp" "${APPLIER_UNIT_PATH}" \
        || fail_step "${EXIT_MUTATION_FAILURE}" write_failed "${APPLIER_UNIT_PATH}" "failed to write the applier unit"
}

# applier_allow_paths <capability> <path>…
# Lets the applier write the given paths while it applies <capability>.
applier_allow_paths() {
    local capability="$1"

    shift

    install -d -m 0755 -o root -g root "${APPLIER_DROPIN_DIR}" \
        || fail_step "${EXIT_MUTATION_FAILURE}" mkdir_failed "${APPLIER_DROPIN_DIR}" "failed to create the applier drop-in directory"

    printf '[Service]\nReadWritePaths=%s\n' "$*" > "${APPLIER_DROPIN_DIR}/${capability}.conf.tmp"
    chmod 0644 "${APPLIER_DROPIN_DIR}/${capability}.conf.tmp"
    chown root:root "${APPLIER_DROPIN_DIR}/${capability}.conf.tmp"
    mv -f "${APPLIER_DROPIN_DIR}/${capability}.conf.tmp" "${APPLIER_DROPIN_DIR}/${capability}.conf" \
        || fail_step "${EXIT_MUTATION_FAILURE}" write_failed "${APPLIER_DROPIN_DIR}/${capability}.conf" "failed to write the applier drop-in for ${capability}"
}

# applier_allow_capabilities <capability> <linux capability>…
# Adds Linux capabilities to what the applier keeps while it applies <capability> (the unit starts
# with a small set). nginx -t binds the listening ports, so nginx needs CAP_NET_BIND_SERVICE.
applier_allow_capabilities() {
    local capability="$1"

    shift

    install -d -m 0755 -o root -g root "${APPLIER_DROPIN_DIR}" \
        || fail_step "${EXIT_MUTATION_FAILURE}" mkdir_failed "${APPLIER_DROPIN_DIR}" "failed to create the applier drop-in directory"

    printf '[Service]\nCapabilityBoundingSet=%s\n' "$*" > "${APPLIER_DROPIN_DIR}/${capability}-capabilities.conf.tmp"
    chmod 0644 "${APPLIER_DROPIN_DIR}/${capability}-capabilities.conf.tmp"
    chown root:root "${APPLIER_DROPIN_DIR}/${capability}-capabilities.conf.tmp"
    mv -f "${APPLIER_DROPIN_DIR}/${capability}-capabilities.conf.tmp" "${APPLIER_DROPIN_DIR}/${capability}-capabilities.conf" \
        || fail_step "${EXIT_MUTATION_FAILURE}" write_failed "${APPLIER_DROPIN_DIR}/${capability}-capabilities.conf" "failed to write the applier capabilities drop-in for ${capability}"
}

# applier_start enables the unit and restarts it so it runs the binary and drop-ins now on disk.
applier_start() {
    # A failure here is a failed health check of the agent binary just installed (the applier is
    # that binary), so it rolls the binary back like a failed self-test does.
    systemctl daemon-reload || agent_fail_selftest_with_rollback "${EXIT_HEALTH_FAILURE}" systemctl_failed "" "systemctl daemon-reload failed"
    systemctl enable lesta-applier >/dev/null 2>&1 || agent_fail_selftest_with_rollback "${EXIT_HEALTH_FAILURE}" systemctl_enable_failed "${APPLIER_UNIT_PATH}" "systemctl enable lesta-applier failed"
    systemctl restart lesta-applier || agent_fail_selftest_with_rollback "${EXIT_HEALTH_FAILURE}" applier_start_failed "${APPLIER_UNIT_PATH}" "systemctl restart lesta-applier failed with the agent binary just installed"

    local waited=0

    while [ ! -S /run/lesta/applier.sock ]; do
        waited=$((waited + 1))

        if [ "${waited}" -gt 50 ]; then
            agent_fail_selftest_with_rollback "${EXIT_HEALTH_FAILURE}" applier_socket_missing /run/lesta/applier.sock "the applier started but its socket did not appear within five seconds"
        fi

        sleep 0.1
    done
}

# applier_enable_capability <capability>
# Adds the capability to the root-owned list the daemon reads, so it hands that capability to the
# applier. Removing the line moves the capability back to the daemon.
applier_enable_capability() {
    local capability="$1"

    if [ -f "${APPLIER_CAPABILITIES_PATH}" ] && grep -qxF "${capability}" "${APPLIER_CAPABILITIES_PATH}"; then
        return 0
    fi

    printf '%s\n' "${capability}" >> "${APPLIER_CAPABILITIES_PATH}"
    chmod 0644 "${APPLIER_CAPABILITIES_PATH}"
    chown root:root "${APPLIER_CAPABILITIES_PATH}"
}

# applier_restart_if_enabled restarts the applier after a new agent binary was installed, so the
# self-test that follows runs against the new applier. A no-op where it is not installed.
applier_restart_if_enabled() {
    if ! systemctl is-enabled lesta-applier >/dev/null 2>&1; then
        return 0
    fi

    systemctl restart lesta-applier || agent_fail_selftest_with_rollback "${EXIT_HEALTH_FAILURE}" applier_restart_failed "${APPLIER_UNIT_PATH}" "systemctl restart lesta-applier failed after installing a new agent binary generation"
}

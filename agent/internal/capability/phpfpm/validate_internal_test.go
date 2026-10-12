package phpfpm

import (
	"strings"
	"testing"
)

func TestTheSyntheticConfigWritesItsErrorLogToTheScratchDirectory(t *testing.T) {
	base := strings.Split("[global]\n;error_log = /var/log/commented.log\npid = /run/php/php8.3-fpm.pid\nerror_log = /var/log/php8.3-fpm.log\ninclude=/etc/php/8.3/fpm/pool.d/*.conf", "\n")

	got := redirectGlobalLog(base, "/tmp/scratch/php-fpm.log")

	want := "[global]\n;error_log = /var/log/commented.log\npid = /run/php/php8.3-fpm.pid\nerror_log = /tmp/scratch/php-fpm.log\ninclude=/etc/php/8.3/fpm/pool.d/*.conf"
	if strings.Join(got, "\n") != want {
		t.Errorf("unexpected configuration:\n%s", strings.Join(got, "\n"))
	}

	if strings.Join(base, "\n") == strings.Join(got, "\n") {
		t.Errorf("the input must not be returned unchanged")
	}
}

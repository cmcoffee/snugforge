package nfo

// export_syslog is the SyslogWriter instance used for exporting logs.
var export_syslog SyslogWriter

// SyslogWriter defines an interface for writing syslog messages.
// It provides methods for different severity levels.
type SyslogWriter interface {
	Alert(string) error
	Crit(string) error
	Debug(string) error
	Emerg(string) error
	Err(string) error
	Info(string) error
	Notice(string) error
	Warning(string) error
}

// HookSyslog replaces the default syslog writer with a custom one.
// It's protected by a mutex for concurrent safety.
func HookSyslog(syslog_writer SyslogWriter) {
	mutex.Lock()
	defer mutex.Unlock()
	export_syslog = syslog_writer
}

// UnhookSyslog resets the syslog writer to its default state.
// It removes any custom syslog writer that was previously set.
func UnhookSyslog() {
	mutex.Lock()
	defer mutex.Unlock()
	export_syslog = nil
}

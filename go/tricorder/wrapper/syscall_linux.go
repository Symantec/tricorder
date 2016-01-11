package wrapper

import (
	"syscall"
)

type Timeval struct {
	Sec  int64
	Usec int64
}

type Rusage struct {
	Utime    Timeval
	Stime    Timeval
	Maxrss   int64
	Ixrss    int64
	Idrss    int64
	Isrss    int64
	Minflt   int64
	Majflt   int64
	Nswap    int64
	Inblock  int64
	Oublock  int64
	Msgsnd   int64
	Msgrcv   int64
	Nsignals int64
	Nvcsw    int64
	Nivcsw   int64
}

func Getrusage(who int, rusage *Rusage) error {
	var syscallRusage syscall.Rusage
	if err := syscall.Getrusage(who, &syscallRusage); err != nil {
		return err
	}
	rusage.Utime.Sec = int64(syscallRusage.Utime.Sec)
	rusage.Utime.Usec = int64(syscallRusage.Utime.Usec)
	rusage.Stime.Sec = int64(syscallRusage.Stime.Sec)
	rusage.Stime.Usec = int64(syscallRusage.Stime.Usec)
	rusage.Maxrss = int64(syscallRusage.Maxrss) << 10
	rusage.Ixrss = int64(syscallRusage.Ixrss) << 10
	rusage.Idrss = int64(syscallRusage.Idrss) << 10
	rusage.Minflt = int64(syscallRusage.Minflt)
	rusage.Majflt = int64(syscallRusage.Majflt)
	rusage.Nswap = int64(syscallRusage.Nswap)
	rusage.Inblock = int64(syscallRusage.Inblock)
	rusage.Oublock = int64(syscallRusage.Oublock)
	rusage.Msgsnd = int64(syscallRusage.Msgsnd)
	rusage.Msgrcv = int64(syscallRusage.Msgrcv)
	rusage.Nsignals = int64(syscallRusage.Nsignals)
	rusage.Nvcsw = int64(syscallRusage.Nvcsw)
	rusage.Nivcsw = int64(syscallRusage.Nivcsw)
	return nil
}

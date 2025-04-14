//go:build windows

package main

import (
	"golang.org/x/sys/windows/svc"
)

type marude_service struct {
	cfg        *CfgData
	caseStatus map[string]*RunStatus
	//wait_group sync.WaitGroup
}

func (m *marude_service) Execute(args []string, r <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	status <- svc.Status{State: svc.StartPending}
	go fiber_service(m.cfg, m.caseStatus)

	status <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				return true, 0
			default:
				Glogger.Infoln("Unexpected service control request #%d", c)
			}
		}
	}
	return false, 1
}

func Init_service(cfg *CfgData, caseStatus map[string]*RunStatus) {
	isService, err := svc.IsWindowsService()
	if err != nil {
		Glogger.Fatal("detect service type failed: %v\n", err)
	}
	if !isService {
		Glogger.Infoln("this process cannot be a windows service, change to console process")
		fiber_service(cfg, caseStatus)
		return
	}

	ws := &marude_service{
		cfg:        cfg,
		caseStatus: caseStatus,
	}

	err = svc.Run("marude_client", ws)
	if err != nil {
		Glogger.Fatalln("run maude client service failed")
	}
	Glogger.Info("finished service\n")
}

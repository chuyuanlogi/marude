//go:build !windows

package main

func Init_service(cfg *CfgData, caseStatus map[string]*RunStatus) {
	fiber_service(cfg, caseStatus)
}

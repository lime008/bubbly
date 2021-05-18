package builtin

import "fmt"

func ReleaseStatusByStages(release Release) string {
	for _, stage := range release.ReleaseStage {
		status := ReleaseStageStatus(stage)
		if status != "READY" {
			return status
		}
	}
	return "READY"
}

func ReleaseStageStatus(stage ReleaseStage) string {
	for _, criteria := range stage.ReleaseCriteria {
		status := ReleaseCriteriaStatus(criteria)
		if status != "READY" {
			return status
		}
	}
	return "READY"
}

func ReleaseCriteriaStatus(criteria ReleaseCriteria) string {
	fmt.Printf("criteria: %#v\n", criteria)
	fmt.Printf("criteria entry: %#v\n", criteria.ReleaseEntry)
	if criteria.ReleaseEntry == nil {
		return "PENDING"
	}
	if !criteria.ReleaseEntry.Result {
		return "BLOCKED"
	}
	return "READY"
}

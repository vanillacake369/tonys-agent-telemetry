package signal

// SignalType* aliases provide alternative constant names used by the trends
// package tests. They are identical to the primary Signal* constants.
// Added in Phase κ to resolve the naming convention used in trends/types_test.go.
const (
	SignalTypeStalledNode           = SignalStalledNode
	SignalTypeDuplicateSubagentWork = SignalDuplicateSubagentWork
	SignalTypeUnusedInstalledSkill  = SignalUnusedInstalledSkill
	SignalTypeFailedHandoff         = SignalFailedHandoff
)

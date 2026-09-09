package workspace

import "testing"

func TestRuleSkillSeparationRuleCopy(t *testing.T) {
	TestCreateSpaceOKF(t)
	TestCreateSpaceOKFFallbackAndInvalid(t)
}

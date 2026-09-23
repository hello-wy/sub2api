package service

// NormalizeIntelligentTestConfigForType keeps pelican runs as drawing previews,
// including settings saved before its answer-grading controls were removed.
// Other test types keep their configured evaluation rules.
func NormalizeIntelligentTestConfigForType(kind string, cfg *IntelligentTestConfig) {
	if kind != "pelican" || cfg == nil {
		return
	}
	cfg.Evaluator = "svg_structure"
	cfg.ExpectedAnswer = ""
	cfg.AnswerType = ""
	cfg.AnswerUnit = ""
	cfg.AnswerUnitMode = ""
	cfg.AnswerFormat = ""
}

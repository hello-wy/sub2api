package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserAttributeNumberValidationAcceptsDecimals(t *testing.T) {
	minValue := 0
	maxValue := 100
	svc := &UserAttributeService{}
	def := &UserAttributeDefinition{
		Name: "积分",
		Type: AttributeTypeNumber,
		Validation: UserAttributeValidation{
			Min: &minValue,
			Max: &maxValue,
		},
	}

	require.NoError(t, svc.validateValue(def, "12.5"))
	require.NoError(t, svc.validateValue(def, "0"))
	require.NoError(t, svc.validateValue(def, "100"))
}

func TestUserAttributeNumberValidationRejectsInvalidNumbers(t *testing.T) {
	minValue := 0
	maxValue := 100
	svc := &UserAttributeService{}
	def := &UserAttributeDefinition{
		Name: "积分",
		Type: AttributeTypeNumber,
		Validation: UserAttributeValidation{
			Min: &minValue,
			Max: &maxValue,
		},
	}

	for _, value := range []string{"abc", "NaN", "101", "-1"} {
		t.Run(value, func(t *testing.T) {
			require.Error(t, svc.validateValue(def, value))
		})
	}
}

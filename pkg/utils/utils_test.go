package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIntersects(t *testing.T) {
	testCases := []struct {
		set1     []string
		set2     []string
		expected bool
	}{
		{
			[]string{"a", "b", "c"},
			[]string{"a1", "b1", "c1"},
			false,
		},
		{
			[]string{"a", "b", "c"},
			[]string{"a1", "b", "c1"},
			true,
		},
		{
			[]string{"a", "b", "c"},
			[]string{"a", "b", "c"},
			true,
		},
		{
			[]string{},
			[]string{},
			false,
		},
	}

	for _, testCase := range testCases {
		actual := Intersects(testCase.set1, testCase.set2)
		assert.Equal(t, testCase.expected, actual)
	}
}

func TestContainsAndTrimPrefix(t *testing.T) {
	testCases := []struct {
		prefixes         []string
		input            string
		expectedTrim     string
		expectedContains bool
	}{
		{
			prefixes:         []string{},
			input:            "build-123",
			expectedTrim:     "build-123",
			expectedContains: false,
		},
		{
			prefixes:         []string{"aviasales/build-"},
			input:            "aviasales/build-123",
			expectedTrim:     "123",
			expectedContains: true,
		},
		{
			prefixes:         []string{"aviasales/build-", "aviasales/back-"},
			input:            "aviasales/back-123",
			expectedTrim:     "123",
			expectedContains: true,
		},
		{
			prefixes:         []string{"aviasales/build-", "aviasales/back-"},
			input:            "aviasales/delta-123",
			expectedTrim:     "aviasales/delta-123",
			expectedContains: false,
		},
		{
			prefixes:         []string{"ab", "a"},
			input:            "aca",
			expectedTrim:     "ca",
			expectedContains: true,
		},
	}

	for _, testCase := range testCases {
		actualTrim, actualContains := ContainsAndTrimPrefix(testCase.input, testCase.prefixes)
		assert.Equal(t, testCase.expectedContains, actualContains)
		assert.Equal(t, testCase.expectedTrim, actualTrim)
	}
}

func TestGetFirstNotNil(t *testing.T) {
	num1, num2 := 1, 2
	var n0 *int
	n1, n2 := &num1, &num2
	assert.Equal(t, *n1, *GetFirstNotNil(n0, n1))
	assert.Equal(t, *n1, *GetFirstNotNil(n1, n0))
	assert.Equal(t, *n2, *GetFirstNotNil(n2, n1))
	assert.Equal(t, *n1, *GetFirstNotNil(n1, n2))
	assert.Equal(t, n0, GetFirstNotNil(n0, n0))
}

func TestGetRepositoryAndTag(t *testing.T) {
	testCases := []struct {
		image              string
		expectedRepository string
		expectedTag        string
	}{
		{
			image:              "aviasales/marketing-samokat:master-432",
			expectedRepository: "aviasales/marketing-samokat",
			expectedTag:        "master-432",
		},
		{
			image:              "/aviasales/marketing-samokat:master-432",
			expectedRepository: "aviasales/marketing-samokat",
			expectedTag:        "master-432",
		},
		{
			image:              "marketing-samokat",
			expectedRepository: "marketing-samokat",
			expectedTag:        "latest",
		},
	}

	for _, testCase := range testCases {
		actualRepository, actualTag := GetRepositoryAndTag(testCase.image)
		assert.Equal(t, testCase.expectedRepository, actualRepository)
		assert.Equal(t, testCase.expectedTag, actualTag)
	}
}

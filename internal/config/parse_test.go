// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("parseKeyValuePairs", func() {

	DescribeTable("parsing key-value pairs",
		func(input []string, expected map[string]string) {
			got := parseKeyValuePairs(input)
			Expect(got).To(Equal(expected))
		},
		Entry("empty input", []string{}, map[string]string{}),
		Entry("single key-value pair", []string{"foo=bar"}, map[string]string{"foo": "bar"}),
		Entry("multiple key-value pairs", []string{"foo=bar", "baz=qux"}, map[string]string{"foo": "bar", "baz": "qux"}),
		Entry("key with empty value", []string{"foo="}, map[string]string{"foo": ""}),
		Entry("key without equal sign", []string{"foo"}, map[string]string{"foo": ""}),
		Entry("value with equal sign", []string{"foo=bar=baz"}, map[string]string{"foo": "bar=baz"}),
		Entry("mixed valid and invalid pairs", []string{"a=1", "b", "c=3"}, map[string]string{"a": "1", "b": "", "c": "3"}),
	)
})

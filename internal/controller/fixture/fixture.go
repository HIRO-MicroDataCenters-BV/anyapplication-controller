// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package fixture

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func SaveStringFixture(filename string, value string) error {
	path := filepath.Join("testdata", filename)
	return os.WriteFile(path, []byte(value), 0644)
}

func LoadStringFixture(filename string) string {
	path := filepath.Join("testdata", filename)
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("failed to read fixture file %s: %v", filename, err))
	}
	return string(raw)
}

func LoadJSONFixture[T any](filename string) T {

	var data T

	path := filepath.Join("testdata", filename)
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("failed to read fixture file %s: %v", filename, err))
	}

	if err := json.Unmarshal(raw, &data); err != nil {
		panic(fmt.Sprintf("failed to unmarshal JSON fixture %s: %v", filename, err))
	}

	return data
}

func LoadYamlFixture(filename string) []*unstructured.Unstructured {

	data := make([]*unstructured.Unstructured, 0)

	path := filepath.Join("testdata", filename)
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("failed to read fixture file %s: %v", filename, err))
	}

	docs := bytes.Split(raw, []byte("\n---"))
	for i, d := range docs {
		jsonData, err := yaml.YAMLToJSON(d)
		if err != nil {
			log.Fatalf("Failed to convert YAML to JSON for resource #%d: %v", i+1, err)
		}

		var obj unstructured.Unstructured
		if err := json.Unmarshal(jsonData, &obj.Object); err != nil {
			log.Fatalf("Failed to unmarshal JSON into unstructured for resource #%d: %v", i+1, err)
		}
		data = append(data, &obj)
	}

	return data
}

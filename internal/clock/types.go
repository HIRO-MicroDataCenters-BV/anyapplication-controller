// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package clock

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Clock interface {
	NowTime() metav1.Time
}

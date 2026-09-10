// Copyright 2024-2026 The NATS Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"fmt"
	"time"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/natscli/columns"
)

// RenderMetaApi draws the _nats.* metadata on streams and consumers
func RenderMetaApi(cols *columns.Writer, metadata map[string]string) {
	versionMeta := metadata[api.JSMetaCurrentServerVersion]
	levelMeta := metadata[api.JSMetaCurrentServerLevel]
	requiredMeta := metadata[api.JsMetaRequiredServerLevel]

	if versionMeta != "" || levelMeta != "" || requiredMeta != "" {
		if versionMeta != "" {
			cols.AddRow("Host Version", versionMeta)
		}

		if levelMeta != "" || requiredMeta != "" {
			if requiredMeta == "" {
				requiredMeta = "0"
			}
			cols.AddRowf("Required API Level", "%s hosted at level %s", requiredMeta, levelMeta)
		}
	}
}

// desiredPlacementValues renders a placement as its tags and preferred server, the cluster
// is reported using the Cluster row, a nil placement or unset field renders as an
// empty string
func desiredPlacementValues(p *api.Placement) (tags string, preferred string) {
	if p == nil {
		return "", ""
	}
	if len(p.Tags) > 0 {
		tags = columns.F(p.Tags)
	}

	return tags, p.Preferred
}

// addDesiredChangedRow adds a "from -> to" row when the values differ, empty values render as none
func addDesiredChangedRow(cols *columns.Writer, title string, from string, to string) {
	if from == to {
		return
	}
	if from == "" {
		from = "none"
	}
	if to == "" {
		to = "none"
	}

	cols.AddRowf(title, "%s -> %s", from, to)
}

// RenderDesiredState renders cluster desired state for streams and consumers
func RenderDesiredState(cols *columns.Writer, desired *api.DesiredClusterInfo, replicas int, placement *api.Placement, retention *api.RetentionPolicy, cluster *api.ClusterInfo, ts time.Time) {
	cols.Indent(3)
	defer cols.Indent(0)

	cols.AddSectionTitle("Cluster Migration Status")
	if desired.Status != nil {
		cols.AddRowIfNotEmpty("Status", desired.Status.Description)
		cols.AddRowIfNotEmpty("Type", columns.F(desired.Status.Type))
		cols.AddRowIfNotEmpty("Error", desired.Status.Err)
	}
	cols.AddRowf("Created", "%s (%s)", columns.F(desired.Created), columns.F(SinceRefOrNow(ts, desired.Created)))
	cols.Println()
	if desired.Name != cluster.Name {
		cols.AddRowf("Cluster", "%s -> %s", cluster.Name, desired.Name)
	}
	if desired.Origin != nil {
		if desired.Origin.Replicas != replicas {
			cols.AddRowf("Replicas", "%d -> %d", desired.Origin.Replicas, replicas)
		}

		ot, op := desiredPlacementValues(desired.Origin.Placement)
		nt, np := desiredPlacementValues(placement)
		addDesiredChangedRow(cols, "Placement Tags", ot, nt)
		addDesiredChangedRow(cols, "Preferred", op, np)
		if desired.Origin.Retention != nil && (retention != nil && *retention != *desired.Origin.Retention) {
			cols.AddRowf("Retention Policy", "%s -> %s", desired.Origin.Retention.String(), retention.String())
		}
	}

	if len(desired.Replicas) > 0 {
		peers := []string{}
		for _, replica := range desired.Replicas {
			if replica.Offline {
				peers = append(peers, fmt.Sprintf("%s (offline)", replica.Name))
			} else {
				peers = append(peers, replica.Name)
			}
		}

		if len(peers) > 0 {
			cols.AddStringsAsValue("Desired Peers", peers)
		}
	}
}

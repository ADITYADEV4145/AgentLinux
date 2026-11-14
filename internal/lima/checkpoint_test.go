// SPDX-License-Identifier: Apache-2.0
package lima

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"github.com/ADITYADEV4145/AgentLinux/internal/config"
	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"go.yaml.in/yaml/v3"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestHelperUpgradeChangesOnlyPackedBinary(t *testing.T) {
	original, e := Template(config.Default(), protocol.ID(), "/fixture/image", "fixture-private-key", "fixture-authorized", []byte("original helper"))
	if e != nil {
		t.Fatal(e)
	}
	updated, e := replaceHelper(original, []byte("replacement helper"))
	if e != nil {
		t.Fatal(e)
	}
	var before, after map[string]any
	_ = yaml.Unmarshal(original, &before)
	_ = yaml.Unmarshal(updated, &after)
	oldProvision := before["provision"].([]any)
	newProvision := after["provision"].([]any)
	oldScript := oldProvision[0].(map[string]any)["script"].(string)
	newScript := newProvision[0].(map[string]any)["script"].(string)
	if helperPayload.ReplaceAllString(oldScript, "HELPER") != helperPayload.ReplaceAllString(newScript, "HELPER") {
		t.Fatal("upgrade changed keys, provisioning recipe or desktop")
	}
	payload := helperPayload.FindStringSubmatch(newScript)[2]
	packed, e := base64.StdEncoding.DecodeString(strings.ReplaceAll(payload, "\n", ""))
	if e != nil {
		t.Fatal(e)
	}
	reader, e := gzip.NewReader(bytes.NewReader(packed))
	if e != nil {
		t.Fatal(e)
	}
	binary, e := io.ReadAll(reader)
	reader.Close()
	if e != nil || string(binary) != "replacement helper" {
		t.Fatal("wrong payload", e)
	}
	delete(before, "provision")
	delete(after, "provision")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("upgrade changed runtime resource/isolation settings")
	}
}

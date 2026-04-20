//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func setupStreamRouteEnvWithKey(g Gomega, apiKey string) []string {
	env := []string{"A6_CONFIG_DIR=" + GinkgoT().TempDir()}
	_, stderr, err := runA6WithEnv(env, "context", "create", "test", "--server", adminURL, "--api-key", apiKey)
	g.Expect(err).NotTo(HaveOccurred(), "failed to create stream-route test context: %s", stderr)
	return env
}

func writeStreamRouteFile(g Gomega, name, body string) string {
	path := filepath.Join(GinkgoT().TempDir(), name)
	g.Expect(os.WriteFile(path, []byte(body), 0o644)).To(Succeed())
	return path
}

func deleteStreamRouteViaAdminByID(g Gomega, id string) {
	resp, err := adminAPI("DELETE", "/apisix/admin/stream_routes/"+id, nil)
	if err == nil && resp != nil {
		g.Expect(resp.Body.Close()).To(Succeed())
	}
}

func deleteStreamRouteViaCLIByID(env []string, id string) {
	_, _, _ = runA6WithEnv(env, "stream-route", "delete", id, "--force")
}

var _ = Describe("stream-route command", Ordered, func() {
	var env []string

	BeforeEach(func() {
		env = setupStreamRouteEnvWithKey(NewWithT(GinkgoT()), adminKey)
	})

	Describe("create", func() {
		It("creates stream routes from json and yaml files against the real Admin API", func() {
			g := NewWithT(GinkgoT())
			jsonID := "ginkgo-stream-route-json"
			yamlID := "ginkgo-stream-route-yaml"

			deleteStreamRouteViaCLIByID(env, jsonID)
			deleteStreamRouteViaCLIByID(env, yamlID)
			DeferCleanup(deleteStreamRouteViaAdminByID, g, jsonID)
			DeferCleanup(deleteStreamRouteViaAdminByID, g, yamlID)

			jsonFile := writeStreamRouteFile(g, "stream-route.json", `{
  "id": "`+jsonID+`",
  "name": "ginkgo-stream-json",
  "server_port": 19100,
  "labels": {"suite":"ginkgo"},
  "upstream": {
    "type": "roundrobin",
    "nodes": {
      "127.0.0.1:8080": 1
    }
  }
}`)
			stdout, stderr, err := runA6WithEnv(env, "stream-route", "create", "-f", jsonFile)
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring(jsonID))

			yamlFile := writeStreamRouteFile(g, "stream-route.yaml", `id: `+yamlID+`
name: ginkgo-stream-yaml
server_port: 19101
upstream:
  type: roundrobin
  nodes:
    127.0.0.1:8080: 1
`)
			stdout, stderr, err = runA6WithEnv(env, "stream-route", "create", "-f", yamlFile, "--output", "yaml")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("name: ginkgo-stream-yaml"))

			_, stderr, err = runA6WithEnv(env, "stream-route", "create")
			g.Expect(err).To(HaveOccurred())
			g.Expect(stderr).To(ContainSubstring("required flag"))
		})
	})

	Describe("list and export", func() {
		It("lists stream routes in table/json and surfaces authentication failures", func() {
			g := NewWithT(GinkgoT())
			id1 := "ginkgo-stream-list-1"
			id2 := "ginkgo-stream-list-2"

			for _, id := range []string{id1, id2} {
				deleteStreamRouteViaCLIByID(env, id)
				DeferCleanup(deleteStreamRouteViaAdminByID, g, id)
			}

			file1 := writeStreamRouteFile(g, "stream-list-1.json", `{
  "id": "`+id1+`",
  "name": "ginkgo-stream-list-1",
  "server_port": 19200,
  "labels": {"env":"test"},
  "upstream": {
    "type": "roundrobin",
    "nodes": {
      "127.0.0.1:8080": 1
    }
  }
}`)
			stdout, stderr, err := runA6WithEnv(env, "stream-route", "create", "-f", file1)
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)

			file2 := writeStreamRouteFile(g, "stream-list-2.json", `{
  "id": "`+id2+`",
  "name": "ginkgo-stream-list-2",
  "server_port": 19201,
  "labels": {"env":"prod"},
  "upstream": {
    "type": "roundrobin",
    "nodes": {
      "127.0.0.1:8080": 1
    }
  }
}`)
			stdout, stderr, err = runA6WithEnv(env, "stream-route", "create", "-f", file2)
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)

			stdout, stderr, err = runA6WithEnv(env, "stream-route", "list", "--output", "table")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("ID"))
			g.Expect(stdout).To(ContainSubstring(id1))
			g.Expect(stdout).To(ContainSubstring(id2))

			stdout, stderr, err = runA6WithEnv(env, "stream-route", "list", "--output", "json")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(json.Valid([]byte(stdout))).To(BeTrue(), stdout)
			g.Expect(stdout).To(ContainSubstring(id1))
			g.Expect(stdout).To(ContainSubstring(id2))

			badEnv := setupStreamRouteEnvWithKey(g, "invalid-api-key")
			_, stderr, err = runA6WithEnv(badEnv, "stream-route", "list")
			g.Expect(err).To(HaveOccurred())
			g.Expect(strings.ToLower(stderr)).To(SatisfyAny(
				ContainSubstring("authentication failed"),
				ContainSubstring("permission denied"),
			))
		})

		It("exports stream routes with real label filtering and strips timestamps", func() {
			g := NewWithT(GinkgoT())
			id1 := "ginkgo-stream-export-1"
			id2 := "ginkgo-stream-export-2"

			for _, id := range []string{id1, id2} {
				deleteStreamRouteViaCLIByID(env, id)
				DeferCleanup(deleteStreamRouteViaAdminByID, g, id)
			}

			file1 := writeStreamRouteFile(g, "stream-export-1.json", `{
  "id": "`+id1+`",
  "name": "ginkgo-stream-export-1",
  "server_port": 19300,
  "labels": {"suite":"ginkgo"},
  "upstream": {
    "type": "roundrobin",
    "nodes": {
      "127.0.0.1:8080": 1
    }
  }
}`)
			stdout, stderr, err := runA6WithEnv(env, "stream-route", "create", "-f", file1)
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)

			file2 := writeStreamRouteFile(g, "stream-export-2.json", `{
  "id": "`+id2+`",
  "name": "ginkgo-stream-export-2",
  "server_port": 19301,
  "labels": {"suite":"other"},
  "upstream": {
    "type": "roundrobin",
    "nodes": {
      "127.0.0.1:8080": 1
    }
  }
}`)
			stdout, stderr, err = runA6WithEnv(env, "stream-route", "create", "-f", file2)
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)

			stdout, stderr, err = runA6WithEnv(env, "stream-route", "export", "--label", "suite=ginkgo", "--output", "json")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring(id1))
			g.Expect(stdout).NotTo(ContainSubstring(id2))
			g.Expect(stdout).NotTo(ContainSubstring("create_time"))
			g.Expect(stdout).NotTo(ContainSubstring("update_time"))

			outFile := filepath.Join(GinkgoT().TempDir(), "stream-export.yaml")
			stdout, stderr, err = runA6WithEnv(env, "stream-route", "export", "-f", outFile, "--output", "yaml")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)

			exported, readErr := os.ReadFile(outFile)
			g.Expect(readErr).NotTo(HaveOccurred())
			g.Expect(string(exported)).To(ContainSubstring(id1))
			g.Expect(string(exported)).To(ContainSubstring(id2))
			g.Expect(string(exported)).NotTo(ContainSubstring("create_time"))
			g.Expect(string(exported)).NotTo(ContainSubstring("update_time"))
		})
	})

	Describe("get, update, and delete", func() {
		It("gets stream routes in yaml/json, updates them, and deletes them against the real Admin API", func() {
			g := NewWithT(GinkgoT())
			streamRouteID := "ginkgo-stream-lifecycle"

			deleteStreamRouteViaCLIByID(env, streamRouteID)
			DeferCleanup(deleteStreamRouteViaAdminByID, g, streamRouteID)

			createFile := writeStreamRouteFile(g, "stream-lifecycle.json", `{
  "id": "`+streamRouteID+`",
  "name": "ginkgo-stream-lifecycle",
  "server_port": 19400,
  "sni": "stream.lifecycle.example.com",
  "upstream": {
    "type": "roundrobin",
    "nodes": {
      "127.0.0.1:8080": 1
    }
  }
}`)
			stdout, stderr, err := runA6WithEnv(env, "stream-route", "create", "-f", createFile)
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)

			stdout, stderr, err = runA6WithEnv(env, "stream-route", "get", streamRouteID, "--output", "yaml")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("name: ginkgo-stream-lifecycle"))
			g.Expect(stdout).To(ContainSubstring("server_port: 19400"))

			stdout, stderr, err = runA6WithEnv(env, "stream-route", "get", streamRouteID, "--output", "json")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(json.Valid([]byte(stdout))).To(BeTrue(), stdout)
			g.Expect(stdout).To(ContainSubstring(streamRouteID))

			updateFile := writeStreamRouteFile(g, "stream-update.json", `{
  "name": "ginkgo-stream-updated",
  "server_port": 19401,
  "upstream": {
    "type": "roundrobin",
    "nodes": {
      "127.0.0.1:8080": 1
    }
  }
}`)
			stdout, stderr, err = runA6WithEnv(env, "stream-route", "update", streamRouteID, "-f", updateFile)
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("ginkgo-stream-updated"))

			stdout, stderr, err = runA6WithEnv(env, "stream-route", "get", streamRouteID, "--output", "json")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("ginkgo-stream-updated"))
			g.Expect(stdout).To(ContainSubstring("19401"))

			stdout, stderr, err = runA6WithEnv(env, "stream-route", "delete", streamRouteID, "--force")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("deleted"))

			_, stderr, err = runA6WithEnv(env, "stream-route", "get", streamRouteID)
			g.Expect(err).To(HaveOccurred())
			g.Expect(strings.ToLower(stderr)).To(ContainSubstring("not found"))
		})
	})
})

func setupStreamRouteEnv(t *testing.T) []string {
	t.Helper()
	env := []string{"A6_CONFIG_DIR=" + t.TempDir()}
	_, _, err := runA6WithEnv(env, "context", "create", "test", "--server", adminURL, "--api-key", adminKey)
	if err != nil {
		t.Fatalf("failed to create stream-route test context: %v", err)
	}
	return env
}

func deleteStreamRouteViaAdmin(t *testing.T, id string) {
	t.Helper()
	resp, err := adminAPI("DELETE", "/apisix/admin/stream_routes/"+id, nil)
	if err == nil && resp != nil {
		_ = resp.Body.Close()
	}
}

//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func setupProtoEnvWithKey(g Gomega, apiKey string) []string {
	env := []string{
		"A6_CONFIG_DIR=" + GinkgoT().TempDir(),
	}
	_, stderr, err := runA6WithEnv(env, "context", "create", "test",
		"--server", adminURL, "--api-key", apiKey)
	g.Expect(err).NotTo(HaveOccurred(), "failed to create proto test context: %s", stderr)
	return env
}

func writeProtoFile(g Gomega, name, body string) string {
	path := filepath.Join(GinkgoT().TempDir(), name)
	g.Expect(os.WriteFile(path, []byte(body), 0o644)).To(Succeed())
	return path
}

func deleteProtoViaAdminByID(g Gomega, id string) {
	resp, err := adminAPI("DELETE", "/apisix/admin/protos/"+id, nil)
	if err == nil && resp != nil {
		g.Expect(resp.Body.Close()).To(Succeed())
	}
}

func deleteProtoViaCLIByID(env []string, id string) {
	_, _, _ = runA6WithEnv(env, "proto", "delete", id, "--force")
}

func createProtoViaCLIFile(g Gomega, env []string, file string) {
	stdout, stderr, err := runA6WithEnv(env, "proto", "create", "-f", file)
	g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
}

var _ = Describe("proto command", Ordered, func() {
	var env []string

	BeforeEach(func() {
		env = setupProtoEnvWithKey(NewWithT(GinkgoT()), adminKey)
	})

	Describe("create", func() {
		It("creates proto definitions from JSON and YAML files against the real Admin API", func() {
			g := NewWithT(GinkgoT())
			jsonID := "ginkgo-proto-json"
			yamlID := "ginkgo-proto-yaml"

			deleteProtoViaCLIByID(env, jsonID)
			deleteProtoViaCLIByID(env, yamlID)
			DeferCleanup(deleteProtoViaAdminByID, g, jsonID)
			DeferCleanup(deleteProtoViaAdminByID, g, yamlID)

			jsonFile := writeProtoFile(g, "proto.json", fmt.Sprintf(`{
  "id": "%s",
  "name": "ginkgo-helloworld",
  "desc": "ginkgo proto json",
  "content": "syntax = \"proto3\";\npackage helloworld;\nmessage HelloRequest { string name = 1; }"
}`, jsonID))
			stdout, stderr, err := runA6WithEnv(env, "proto", "create", "-f", jsonFile)
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring(jsonID))

			resp, apiErr := adminAPI("GET", "/apisix/admin/protos/"+jsonID, nil)
			g.Expect(apiErr).NotTo(HaveOccurred())
			g.Expect(resp.StatusCode).To(Equal(200))
			g.Expect(resp.Body.Close()).To(Succeed())

			yamlFile := writeProtoFile(g, "proto.yaml", fmt.Sprintf(`id: %s
name: ginkgo-echo
desc: ginkgo proto yaml
content: |-
  syntax = "proto3";
  package echo;
  message EchoRequest { string msg = 1; }
`, yamlID))
			stdout, stderr, err = runA6WithEnv(env, "proto", "create", "-f", yamlFile, "--output", "yaml")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("name: ginkgo-echo"))
		})

		It("supports create without explicit id and preserves required-flag validation", func() {
			g := NewWithT(GinkgoT())

			noIDFile := writeProtoFile(g, "proto-no-id.json", `{
  "name": "ginkgo-generated-proto",
  "desc": "proto without explicit id",
  "content": "syntax = \"proto3\";\npackage generated;\nmessage Ping { string message = 1; }"
}`)
			stdout, stderr, err := runA6WithEnv(env, "proto", "create", "-f", noIDFile)
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("ginkgo-generated-proto"))

			_, stderr, err = runA6WithEnv(env, "proto", "create")
			g.Expect(err).To(HaveOccurred())
			g.Expect(stderr).To(ContainSubstring("required flag"))
		})
	})

	Describe("list and export", func() {
		It("renders list output modes and surfaces authentication failures", func() {
			g := NewWithT(GinkgoT())
			id1 := "ginkgo-proto-list-1"
			id2 := "ginkgo-proto-list-2"

			deleteProtoViaCLIByID(env, id1)
			deleteProtoViaCLIByID(env, id2)
			DeferCleanup(deleteProtoViaAdminByID, g, id1)
			DeferCleanup(deleteProtoViaAdminByID, g, id2)

			file1 := writeProtoFile(g, "proto-list-1.json", fmt.Sprintf(`{
  "id": "%s",
  "name": "ginkgo-proto-alpha",
  "desc": "alpha proto",
  "labels": {"type":"grpc"},
  "content": "syntax = \"proto3\";\npackage alpha;\nmessage Alpha { string msg = 1; }"
}`, id1))
			createProtoViaCLIFile(g, env, file1)

			file2 := writeProtoFile(g, "proto-list-2.json", fmt.Sprintf(`{
  "id": "%s",
  "name": "ginkgo-proto-beta",
  "desc": "beta proto",
  "labels": {"type":"echo"},
  "content": "syntax = \"proto3\";\npackage beta;\nmessage Beta { string msg = 1; }"
}`, id2))
			createProtoViaCLIFile(g, env, file2)

			stdout, stderr, err := runA6WithEnv(env, "proto", "list", "--output", "table")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("ID"))
			g.Expect(stdout).To(ContainSubstring(id1))
			g.Expect(stdout).To(ContainSubstring(id2))

			stdout, stderr, err = runA6WithEnv(env, "proto", "list", "--output", "json")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(json.Valid([]byte(stdout))).To(BeTrue(), stdout)
			g.Expect(stdout).To(ContainSubstring(id1))
			g.Expect(stdout).To(ContainSubstring(id2))

			stdout, stderr, err = runA6WithEnv(env, "proto", "list", "--output", "yaml")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("id: " + id1))
			g.Expect(stdout).To(ContainSubstring("id: " + id2))

			badEnv := setupProtoEnvWithKey(g, "invalid-api-key")
			_, stderr, err = runA6WithEnv(badEnv, "proto", "list")
			g.Expect(err).To(HaveOccurred())
			g.Expect(strings.ToLower(stderr)).To(SatisfyAny(
				ContainSubstring("authentication failed"),
				ContainSubstring("permission denied"),
			))
		})

		It("exports proto definitions with real label filtering and strips timestamps from output", func() {
			g := NewWithT(GinkgoT())
			id1 := "ginkgo-proto-export-1"
			id2 := "ginkgo-proto-export-2"

			deleteProtoViaCLIByID(env, id1)
			deleteProtoViaCLIByID(env, id2)
			DeferCleanup(deleteProtoViaAdminByID, g, id1)
			DeferCleanup(deleteProtoViaAdminByID, g, id2)

			file1 := writeProtoFile(g, "proto-export-1.json", fmt.Sprintf(`{
  "id": "%s",
  "name": "ginkgo-proto-export-1",
  "labels": {"type":"grpc"},
  "content": "syntax = \"proto3\";\npackage export1;\nmessage Export1 { string msg = 1; }"
}`, id1))
			createProtoViaCLIFile(g, env, file1)

			file2 := writeProtoFile(g, "proto-export-2.json", fmt.Sprintf(`{
  "id": "%s",
  "name": "ginkgo-proto-export-2",
  "labels": {"type":"echo"},
  "content": "syntax = \"proto3\";\npackage export2;\nmessage Export2 { string msg = 1; }"
}`, id2))
			createProtoViaCLIFile(g, env, file2)

			stdout, stderr, err := runA6WithEnv(env, "proto", "export", "--label", "type=grpc", "--output", "json")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring(id1))
			g.Expect(stdout).NotTo(ContainSubstring(id2))
			g.Expect(stdout).NotTo(ContainSubstring("create_time"))
			g.Expect(stdout).NotTo(ContainSubstring("update_time"))

			outFile := filepath.Join(GinkgoT().TempDir(), "proto-export.yaml")
			stdout, stderr, err = runA6WithEnv(env, "proto", "export", "-f", outFile, "--output", "yaml")
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
		It("gets proto definitions in yaml/json, updates them, and deletes them against the real Admin API", func() {
			g := NewWithT(GinkgoT())
			protoID := "ginkgo-proto-lifecycle"

			deleteProtoViaCLIByID(env, protoID)
			DeferCleanup(deleteProtoViaAdminByID, g, protoID)

			createFile := writeProtoFile(g, "proto-lifecycle.json", fmt.Sprintf(`{
  "id": "%s",
  "name": "ginkgo-proto-lifecycle",
  "desc": "lifecycle proto",
  "content": "syntax = \"proto3\";\npackage lifecycle;\nmessage Hello { string name = 1; }"
}`, protoID))
			createProtoViaCLIFile(g, env, createFile)

			stdout, stderr, err := runA6WithEnv(env, "proto", "get", protoID, "--output", "yaml")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("name: ginkgo-proto-lifecycle"))
			g.Expect(stdout).To(ContainSubstring("desc: lifecycle proto"))

			stdout, stderr, err = runA6WithEnv(env, "proto", "get", protoID, "--output", "json")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(json.Valid([]byte(stdout))).To(BeTrue(), stdout)
			g.Expect(stdout).To(ContainSubstring(protoID))

			updateFile := writeProtoFile(g, "proto-update.json", `{
  "name": "ginkgo-proto-updated",
  "desc": "updated lifecycle proto",
  "content": "syntax = \"proto3\";\npackage lifecycle;\nmessage Hello { string name = 1; }\nmessage Goodbye { string name = 1; }"
}`)
			stdout, stderr, err = runA6WithEnv(env, "proto", "update", protoID, "-f", updateFile)
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("ginkgo-proto-updated"))

			stdout, stderr, err = runA6WithEnv(env, "proto", "get", protoID, "--output", "json")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("ginkgo-proto-updated"))

			stdout, stderr, err = runA6WithEnv(env, "proto", "delete", protoID, "--force")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("deleted"))

			resp, apiErr := adminAPI("GET", "/apisix/admin/protos/"+protoID, nil)
			g.Expect(apiErr).NotTo(HaveOccurred())
			g.Expect(resp.StatusCode).To(Equal(404))
			g.Expect(resp.Body.Close()).To(Succeed())
		})

		It("surfaces get and delete not-found behavior and update required-flag validation through the real CLI", func() {
			g := NewWithT(GinkgoT())
			protoID := "nonexistent-proto-999"

			deleteProtoViaCLIByID(env, protoID)
			DeferCleanup(deleteProtoViaAdminByID, g, protoID)

			_, stderr, err := runA6WithEnv(env, "proto", "get", protoID)
			g.Expect(err).To(HaveOccurred())
			g.Expect(strings.ToLower(stderr)).To(ContainSubstring("not found"))

			_, stderr, err = runA6WithEnv(env, "proto", "update")
			g.Expect(err).To(HaveOccurred())
			g.Expect(stderr).To(ContainSubstring("required flag"))

			_, stderr, err = runA6WithEnv(env, "proto", "delete", protoID, "--force")
			g.Expect(err).To(HaveOccurred())
			g.Expect(strings.ToLower(stderr)).To(ContainSubstring("not found"))
		})
	})
})

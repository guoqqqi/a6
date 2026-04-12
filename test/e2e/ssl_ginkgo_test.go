//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func setupSSLEnvWithKey(g Gomega, apiKey string) []string {
	env := []string{
		"A6_CONFIG_DIR=" + GinkgoT().TempDir(),
	}
	_, stderr, err := runA6WithEnv(env, "context", "create", "test",
		"--server", adminURL, "--api-key", apiKey)
	g.Expect(err).NotTo(HaveOccurred(), "failed to create ssl test context: %s", stderr)
	return env
}

func readTestCertPair(g Gomega) (string, string) {
	modRoot, err := resolveModuleRoot()
	g.Expect(err).NotTo(HaveOccurred())

	certBytes, err := os.ReadFile(filepath.Join(modRoot, "test/e2e/testdata/test.crt"))
	g.Expect(err).NotTo(HaveOccurred())

	keyBytes, err := os.ReadFile(filepath.Join(modRoot, "test/e2e/testdata/test.key"))
	g.Expect(err).NotTo(HaveOccurred())

	return string(certBytes), string(keyBytes)
}

func writeSSLFile(g Gomega, name, body string) string {
	path := filepath.Join(GinkgoT().TempDir(), name)
	g.Expect(os.WriteFile(path, []byte(body), 0o644)).To(Succeed())
	return path
}

func deleteSSLViaAdminByID(g Gomega, id string) {
	resp, err := adminAPI("DELETE", "/apisix/admin/ssls/"+id, nil)
	if err == nil && resp != nil {
		g.Expect(resp.Body.Close()).To(Succeed())
	}
}

func deleteSSLViaCLIByID(env []string, id string) {
	_, _, _ = runA6WithEnv(env, "ssl", "delete", id, "--force")
}

var _ = Describe("ssl command", Ordered, func() {
	var env []string

	BeforeEach(func() {
		env = setupSSLEnvWithKey(NewWithT(GinkgoT()), adminKey)
	})

	Describe("create", func() {
		It("creates SSL resources from json and yaml files against the real Admin API", func() {
			g := NewWithT(GinkgoT())
			cert, key := readTestCertPair(g)
			jsonID := "ginkgo-ssl-json"
			yamlID := "ginkgo-ssl-yaml"

			deleteSSLViaCLIByID(env, jsonID)
			deleteSSLViaCLIByID(env, yamlID)
			DeferCleanup(deleteSSLViaAdminByID, g, jsonID)
			DeferCleanup(deleteSSLViaAdminByID, g, yamlID)

			jsonFile := writeSSLFile(g, "ssl.json", `{
  "id": "`+jsonID+`",
  "cert": `+strconvQuote(cert)+`,
  "key": `+strconvQuote(key)+`,
  "snis": ["json.ssl.example.com"],
  "labels": {"suite":"ginkgo"}
}`)
			stdout, stderr, err := runA6WithEnv(env, "ssl", "create", "-f", jsonFile)
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring(jsonID))

			yamlFile := writeSSLFile(g, "ssl.yaml", "id: "+yamlID+"\ncert: |-\n"+indentBlock(cert, 2)+"\nkey: |-\n"+indentBlock(key, 2)+"\nsnis:\n  - yaml.ssl.example.com\n")
			stdout, stderr, err = runA6WithEnv(env, "ssl", "create", "-f", yamlFile, "--output", "yaml")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("yaml.ssl.example.com"))

			_, stderr, err = runA6WithEnv(env, "ssl", "create")
			g.Expect(err).To(HaveOccurred())
			g.Expect(stderr).To(ContainSubstring("required flag"))
		})
	})

	Describe("list and export", func() {
		It("lists SSL resources in table/json and surfaces authentication failures", func() {
			g := NewWithT(GinkgoT())
			cert, key := readTestCertPair(g)
			id1 := "ginkgo-ssl-list-1"
			id2 := "ginkgo-ssl-list-2"

			for _, id := range []string{id1, id2} {
				deleteSSLViaCLIByID(env, id)
				DeferCleanup(deleteSSLViaAdminByID, g, id)
			}

			file1 := writeSSLFile(g, "ssl-list-1.json", `{
  "id": "`+id1+`",
  "cert": `+strconvQuote(cert)+`,
  "key": `+strconvQuote(key)+`,
  "snis": ["list-1.ssl.example.com"],
  "labels": {"env":"test"},
  "status": 1
}`)
			stdout, stderr, err := runA6WithEnv(env, "ssl", "create", "-f", file1)
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)

			file2 := writeSSLFile(g, "ssl-list-2.json", `{
  "id": "`+id2+`",
  "cert": `+strconvQuote(cert)+`,
  "key": `+strconvQuote(key)+`,
  "snis": ["list-2.ssl.example.com"],
  "labels": {"env":"prod"},
  "status": 1
}`)
			stdout, stderr, err = runA6WithEnv(env, "ssl", "create", "-f", file2)
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)

			stdout, stderr, err = runA6WithEnv(env, "ssl", "list", "--output", "table")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("ID"))
			g.Expect(stdout).To(ContainSubstring(id1))
			g.Expect(stdout).To(ContainSubstring(id2))
			g.Expect(stdout).To(ContainSubstring("VALIDITY"))

			stdout, stderr, err = runA6WithEnv(env, "ssl", "list", "--output", "json")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring(id1))
			g.Expect(stdout).To(ContainSubstring(id2))

			badEnv := setupSSLEnvWithKey(g, "invalid-api-key")
			_, stderr, err = runA6WithEnv(badEnv, "ssl", "list")
			g.Expect(err).To(HaveOccurred())
			g.Expect(strings.ToLower(stderr)).To(SatisfyAny(
				ContainSubstring("authentication failed"),
				ContainSubstring("permission denied"),
			))
		})

		It("exports SSL resources with real label filtering and strips timestamps", func() {
			g := NewWithT(GinkgoT())
			cert, key := readTestCertPair(g)
			id1 := "ginkgo-ssl-export-1"
			id2 := "ginkgo-ssl-export-2"

			for _, id := range []string{id1, id2} {
				deleteSSLViaCLIByID(env, id)
				DeferCleanup(deleteSSLViaAdminByID, g, id)
			}

			file1 := writeSSLFile(g, "ssl-export-1.json", `{
  "id": "`+id1+`",
  "cert": `+strconvQuote(cert)+`,
  "key": `+strconvQuote(key)+`,
  "snis": ["export-1.ssl.example.com"],
  "labels": {"env":"test"}
}`)
			stdout, stderr, err := runA6WithEnv(env, "ssl", "create", "-f", file1)
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)

			file2 := writeSSLFile(g, "ssl-export-2.json", `{
  "id": "`+id2+`",
  "cert": `+strconvQuote(cert)+`,
  "key": `+strconvQuote(key)+`,
  "snis": ["export-2.ssl.example.com"],
  "labels": {"env":"prod"}
}`)
			stdout, stderr, err = runA6WithEnv(env, "ssl", "create", "-f", file2)
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)

			stdout, stderr, err = runA6WithEnv(env, "ssl", "export", "--label", "env=test", "--output", "json")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring(id1))
			g.Expect(stdout).NotTo(ContainSubstring(id2))
			g.Expect(stdout).NotTo(ContainSubstring("create_time"))
			g.Expect(stdout).NotTo(ContainSubstring("update_time"))

			outFile := filepath.Join(GinkgoT().TempDir(), "ssl-export.yaml")
			stdout, stderr, err = runA6WithEnv(env, "ssl", "export", "-f", outFile, "--output", "yaml")
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
		It("gets SSL resources in yaml/json, updates them, and deletes them against the real Admin API", func() {
			g := NewWithT(GinkgoT())
			cert, key := readTestCertPair(g)
			sslID := "ginkgo-ssl-lifecycle"

			deleteSSLViaCLIByID(env, sslID)
			DeferCleanup(deleteSSLViaAdminByID, g, sslID)

			createFile := writeSSLFile(g, "ssl-lifecycle.json", `{
  "id": "`+sslID+`",
  "cert": `+strconvQuote(cert)+`,
  "key": `+strconvQuote(key)+`,
  "snis": ["get-1.ssl.example.com", "get-2.ssl.example.com"]
}`)
			stdout, stderr, err := runA6WithEnv(env, "ssl", "create", "-f", createFile)
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)

			stdout, stderr, err = runA6WithEnv(env, "ssl", "get", sslID)
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("get-1.ssl.example.com"))
			g.Expect(stdout).To(ContainSubstring("get-2.ssl.example.com"))

			stdout, stderr, err = runA6WithEnv(env, "ssl", "get", sslID, "--output", "json")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring(sslID))

			updateFile := writeSSLFile(g, "ssl-update.json", `{
  "cert": `+strconvQuote(cert)+`,
  "key": `+strconvQuote(key)+`,
  "snis": ["updated.ssl.example.com"]
}`)
			stdout, stderr, err = runA6WithEnv(env, "ssl", "update", sslID, "-f", updateFile)
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("updated.ssl.example.com"))

			stdout, stderr, err = runA6WithEnv(env, "ssl", "get", sslID, "--output", "json")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("updated.ssl.example.com"))

			stdout, stderr, err = runA6WithEnv(env, "ssl", "delete", sslID, "--force")
			g.Expect(err).NotTo(HaveOccurred(), "stdout=%s stderr=%s", stdout, stderr)
			g.Expect(stdout).To(ContainSubstring("deleted"))

			_, stderr, err = runA6WithEnv(env, "ssl", "get", sslID)
			g.Expect(err).To(HaveOccurred())
			g.Expect(strings.ToLower(stderr)).To(ContainSubstring("not found"))
		})
	})
})

func indentBlock(s string, spaces int) string {
	indent := strings.Repeat(" ", spaces)
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	return indent + strings.Join(lines, "\n"+indent)
}

func strconvQuote(s string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), "\n", `\n`) + `"`
}

func setupSSLEnv(t *testing.T) []string {
	t.Helper()
	env := []string{
		"A6_CONFIG_DIR=" + t.TempDir(),
	}
	_, _, err := runA6WithEnv(env, "context", "create", "test",
		"--server", adminURL, "--api-key", adminKey)
	if err != nil {
		t.Fatalf("failed to create ssl test context: %v", err)
	}
	return env
}

func deleteSSLViaAdmin(t *testing.T, id string) {
	t.Helper()
	resp, err := adminAPI("DELETE", "/apisix/admin/ssls/"+id, nil)
	if err == nil && resp != nil {
		_ = resp.Body.Close()
	}
}

func readTestCert(t *testing.T) (string, string) {
	t.Helper()
	modRoot, err := resolveModuleRoot()
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}

	certBytes, err := os.ReadFile(filepath.Join(modRoot, "test/e2e/testdata/test.crt"))
	if err != nil {
		t.Fatalf("read test cert: %v", err)
	}

	keyBytes, err := os.ReadFile(filepath.Join(modRoot, "test/e2e/testdata/test.key"))
	if err != nil {
		t.Fatalf("read test key: %v", err)
	}

	return string(certBytes), string(keyBytes)
}

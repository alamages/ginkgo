package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"strings"
	"io/ioutil"
	"fmt"
)

func replaceInPlace(path, oldString, newString string, times int) {
	fileContents, err := ioutil.ReadFile(path)
	Ω(err).NotTo(HaveOccurred())

	newContents := strings.Replace(string(fileContents), oldString, newString, times)

	err = ioutil.WriteFile(path, []byte(newContents), 0)
	Ω(err).NotTo(HaveOccurred())
}

func getGoPathEnvWith(additionalGoPath string) string {
	goPathEnv := fmt.Sprintf("GOPATH=%s", additionalGoPath)

	existingGoPath, exists := os.LookupEnv("GOPATH")
	if exists {
		goPathEnv = fmt.Sprintf("%s:%s", goPathEnv, existingGoPath)
	}

	return goPathEnv
}

var _ = Describe("Coverage Specs", func() {
	var pathToTest string
	var tmpGoPath string
	var sessionEnv []string
	var coverProfilePath string

	BeforeEach(func() {
		tmpGoPath = tmpPath("gopath")
		pathToTest = filepath.Join(tmpGoPath, "src", "coverage_fixture")
		pathToExternalCoverage := filepath.Join(pathToTest, "external_coverage_fixture")
		sessionEnv = os.Environ()
		sessionEnv = append(sessionEnv, getGoPathEnvWith(tmpGoPath))
		coverProfilePath = filepath.Join(pathToTest, "coverage_fixture.coverprofile")

		// clean this up
		copyIn("coverage_fixture", pathToTest)
		copyIn(filepath.Join("coverage_fixture", "external_coverage_fixture"), pathToExternalCoverage)
		err := os.Remove(filepath.Join(pathToTest, "external_coverage.go"))
		Ω(err).ShouldNot(HaveOccurred())

		replaceInPlace(filepath.Join(pathToTest, "coverage_fixture_test.go"), "github.com/onsi/ginkgo/integration/_fixtures/", "", 2)
	})

	AfterEach(func() {
		os.RemoveAll(coverProfilePath)
	})

	It("runs coverage analysis in series and in parallel", func() {
		session := startGinkgoWithEnv(sessionEnv, pathToTest, "-cover")
		//session := startGinkgo(pathToTest, "-cover")
		Eventually(session).Should(gexec.Exit(0))
		output := session.Out.Contents()
		Ω(string(output)).Should(ContainSubstring("coverage: 80.0% of statements"))

		cmd := exec.Command("go", "tool", "cover", fmt.Sprintf("-func=%s", coverProfilePath))
		cmd.Env = sessionEnv
		serialCoverProfileOutput, err := cmd.CombinedOutput()
		Ω(err).ShouldNot(HaveOccurred())

		os.RemoveAll(coverProfilePath)

		Eventually(startGinkgoWithEnv(sessionEnv, pathToTest, "-cover", "-nodes=4")).Should(gexec.Exit(0))

		cmd = exec.Command("go", "tool", "cover", fmt.Sprintf("-func=%s", coverProfilePath))
		cmd.Env = sessionEnv
		parallelCoverProfileOutput, err := cmd.CombinedOutput()
		Ω(err).ShouldNot(HaveOccurred())

		Ω(parallelCoverProfileOutput).Should(Equal(serialCoverProfileOutput))

		By("handling external packages")
		session = startGinkgoWithEnv(sessionEnv, pathToTest, "-coverpkg=coverage_fixture,coverage_fixture/external_coverage_fixture")
		Eventually(session).Should(gexec.Exit(0))
		output = session.Out.Contents()
		Ω(output).Should(ContainSubstring("coverage: 71.4% of statements in coverage_fixture, coverage_fixture/external_coverage_fixture"))

		cmd = exec.Command("go", "tool", "cover", fmt.Sprintf("-func=%s", coverProfilePath))
		cmd.Env = sessionEnv
		serialCoverProfileOutput, err = cmd.CombinedOutput()
		Ω(err).ShouldNot(HaveOccurred())

		os.RemoveAll(coverProfilePath)

		Eventually(startGinkgoWithEnv(sessionEnv, pathToTest, "-coverpkg=coverage_fixture,coverage_fixture/external_coverage_fixture", "-nodes=4")).Should(gexec.Exit(0))

		cmd = exec.Command("go", "tool", "cover", fmt.Sprintf("-func=%s", coverProfilePath))
		cmd.Env = sessionEnv
		parallelCoverProfileOutput, err = cmd.CombinedOutput()
		Ω(err).ShouldNot(HaveOccurred())

		Ω(parallelCoverProfileOutput).Should(Equal(serialCoverProfileOutput))
	})

	It("validates coverprofile sets custom profile name", func() {
		session := startGinkgoWithEnv(sessionEnv, pathToTest, "-cover", "-coverprofile=coverage.txt")

		Eventually(session).Should(gexec.Exit(0))

		// Check that the correct file was created
		coverFile := filepath.Join(pathToTest, "coverage.txt")
		_, err := os.Stat(coverFile)

		Ω(err).ShouldNot(HaveOccurred())

		// Cleanup
		os.RemoveAll(coverFile)
	})

	It("Works in recursive mode", func() {
		session := startGinkgo("./_fixtures/combined_coverage_fixture", "-r", "-cover", "-coverprofile=coverage.txt")

		Eventually(session).Should(gexec.Exit(0))

		packages := []string{"first_package", "second_package"}

		for _, p := range packages {
			coverFile := fmt.Sprintf("./_fixtures/combined_coverage_fixture/%s/coverage.txt", p)
			_, err := os.Stat(coverFile)

			Ω(err).ShouldNot(HaveOccurred())

			// Cleanup
			os.RemoveAll(coverFile)
		}
	})

	It("Works in parallel mode", func() {
		session := startGinkgoWithEnv(sessionEnv, pathToTest, "-p", "-cover", "-coverprofile=coverage.txt")

		Eventually(session).Should(gexec.Exit(0))

		coverFile := filepath.Join(pathToTest, "coverage.txt")
		_, err := os.Stat(coverFile)

		Ω(err).ShouldNot(HaveOccurred())

		// Cleanup
		os.RemoveAll(coverFile)
	})

	XIt("Appends coverages if output dir and coverprofile were set", func() {
		session := startGinkgo("./_fixtures/combined_coverage_fixture", "-outputdir=./", "-r", "-cover", "-coverprofile=coverage.txt")

		Eventually(session).Should(gexec.Exit(0))

		_, err := os.Stat("./_fixtures/combined_coverage_fixture/coverage.txt")

		Ω(err).ShouldNot(HaveOccurred())

		// Cleanup
		os.RemoveAll("./_fixtures/combined_coverage_fixture/coverage.txt")
	})

	XIt("Creates directories in path if they don't exist", func() {
		session := startGinkgo("./_fixtures/combined_coverage_fixture", "-outputdir=./all/profiles/here", "-r", "-cover", "-coverprofile=coverage.txt")

		defer os.RemoveAll("./_fixtures/combined_coverage_fixture/all")
		defer os.RemoveAll("./_fixtures/combined_coverage_fixture/coverage.txt")

		Eventually(session).Should(gexec.Exit(0))

		_, err := os.Stat("./_fixtures/combined_coverage_fixture/all/profiles/here/coverage.txt")

		Ω(err).ShouldNot(HaveOccurred())
	})

	XIt("Moves coverages if only output dir was set", func() {
		session := startGinkgo("./_fixtures/combined_coverage_fixture", "-outputdir=./", "-r", "-cover")

		Eventually(session).Should(gexec.Exit(0))

		packages := []string{"first_package", "second_package"}

		for _, p := range packages {
			coverFile := fmt.Sprintf("./_fixtures/combined_coverage_fixture/%s.coverprofile", p)

			// Cleanup
			defer func(f string) {
				os.RemoveAll(f)
			}(coverFile)

			defer func(f string) {
				os.RemoveAll(fmt.Sprintf("./_fixtures/combined_coverage_fixture/%s/coverage.txt", f))
			}(p)

			_, err := os.Stat(coverFile)

			Ω(err).ShouldNot(HaveOccurred())
		}
	})
})

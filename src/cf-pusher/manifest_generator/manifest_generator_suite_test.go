package manifest_generator_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestManifestGenerator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ManifestGenerator Suite")
}

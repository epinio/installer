package installer_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/epinio/installer/internal/installer"
)

var _ = Describe("Plan", func() {
	It("Validates 'needs' graph to be free of cycles", func() {
		m, err := installer.Load(assetPath("test-manifest.yml"))
		Expect(err).ToNot(HaveOccurred())

		plan, err := installer.BuildPlan(m.Components)
		// Plan finds no cycles
		Expect(err).ToNot(HaveOccurred())
		Expect(plan).To(HaveLen(len(m.Components)))

		// Plan doesn't know about concurrency, though
		Expect(plan.IDs()).To(Equal([]installer.DeploymentID{"epinio-namespace", "linkerd", "traefik", "cert-manager", "cluster-issuers", "cluster-certificates", "tekton", "tekton-pipelines", "kubed", "epinio"}))
	})
})

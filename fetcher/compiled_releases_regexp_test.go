package fetcher_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/internal/cargo"
)

var _ = Describe("CompiledReleasesRegexp", func() {
	var (
		compiledRelease cargo.CompiledRelease
		regex           *fetcher.CompiledReleasesRegexp
		err             error
	)

	It("takes a regex string and converts it to a CompiledRelease", func() {
		regex, err = fetcher.NewCompiledReleasesRegexp(`^2.5/.+/(?P<release_name>[a-z-_]+)-(?P<release_version>[0-9\.]+)-(?P<stemcell_os>[a-z-_]+)-(?P<stemcell_version>[\d\.]+)\.tgz$`)
		Expect(err).NotTo(HaveOccurred())

		compiledRelease, err = regex.Convert("2.5/uaa/uaa-1.2.3-ubuntu-trusty-123.tgz")
		Expect(err).NotTo(HaveOccurred())
		Expect(compiledRelease).To(Equal(cargo.CompiledRelease{Name: "uaa", Version: "1.2.3", StemcellOS: "ubuntu-trusty", StemcellVersion: "123"}))
	})

	It("returns an error if s3 key does not match the regex", func() {
		regex, err = fetcher.NewCompiledReleasesRegexp(`^2.5/.+/(?P<release_name>[a-z-_]+)-(?P<release_version>[0-9\.]+)-(?P<stemcell_os>[a-z-_]+)-(?P<stemcell_version>[\d\.]+)\.tgz$`)
		Expect(err).NotTo(HaveOccurred())

		compiledRelease, err = regex.Convert("2.5/uaa/uaa-1.2.3-123.tgz")
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError("s3 key does not match regex"))
	})

	It("returns an error if a capture is missing", func() {
		regex, err = fetcher.NewCompiledReleasesRegexp(`^2.5/.+/([a-z-_]+)-(?P<release_version>[0-9\.]+)-(?P<stemcell_os>[a-z-_]+)-(?P<stemcell_version>[\d\.]+)\.tgz$`)
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(ContainSubstring("release_name, release_version, stemcell_os, stemcell_version")))
	})
})

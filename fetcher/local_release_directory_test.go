package fetcher_test

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/fetcher"
	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/cargo"
)

var _ = Describe("LocalReleaseDirectory", func() {
	var (
		localReleaseDirectory fetcher.LocalReleaseDirectory
		noConfirm             bool
		releasesDir           string
		releaseFile           string
		fakeLogger            *log.Logger
	)

	BeforeEach(func() {
		var err error
		releasesDir, err = ioutil.TempDir("", "releases")
		noConfirm = true
		Expect(err).NotTo(HaveOccurred())

		releaseFile = filepath.Join(releasesDir, "some-release.tgz")

		fakeLogger = log.New(GinkgoWriter, "", 0)
		releaseManifestReader := builder.NewReleaseManifestReader()
		releasesService := baking.NewReleasesService(fakeLogger, releaseManifestReader)

		localReleaseDirectory = fetcher.NewLocalReleaseDirectory(fakeLogger, releasesService)
	})

	AfterEach(func() {
		_ = os.RemoveAll(releasesDir)
	})

	Describe("GetLocalReleases", func() {
		Context("when releases exist in the releases dir", func() {
			BeforeEach(func() {
				fixtureContent, err := ioutil.ReadFile(filepath.Join("fixtures", "some-release.tgz"))
				Expect(err).NotTo(HaveOccurred())
				err = ioutil.WriteFile(releaseFile, fixtureContent, 0755)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns a map of releases to locations", func() {
				releases, err := localReleaseDirectory.GetLocalReleases(releasesDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(releases).To(HaveLen(1))
				Expect(releases).To(HaveKeyWithValue(cargo.CompiledRelease{
					Name:            "some-release",
					Version:         "1.2.3",
					StemcellOS:      "some-os",
					StemcellVersion: "4.5.6",
				}, releaseFile))
			})
		})

		Context("when there are no local releases", func() {
			It("returns an empty slice", func() {
				releases, err := localReleaseDirectory.GetLocalReleases(releasesDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(releases).To(HaveLen(0))
			})
		})

		Context("when the releases directory does not exist", func() {
			It("returns an empty slice", func() {
				_, err := localReleaseDirectory.GetLocalReleases("some-invalid-directory")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("some-invalid-directory"))
			})
		})
	})

	Describe("DeleteExtraReleases", func() {
		var extraFile *os.File
		BeforeEach(func() {
			var err error
			extraFile, err = ioutil.TempFile(releasesDir, "extra-release")
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes specified files", func() {
			extraRelease := cargo.CompiledRelease{
				Name:            "extra-release",
				Version:         "v0.0",
				StemcellOS:      "os-0",
				StemcellVersion: "v0.0.0",
			}

			extraFileName := extraFile.Name()
			extraReleases := map[cargo.CompiledRelease]string{}
			extraReleases[extraRelease] = extraFileName

			err := localReleaseDirectory.DeleteExtraReleases(releasesDir, extraReleases, noConfirm)
			Expect(err).NotTo(HaveOccurred())

			_, err = os.Stat(extraFile.Name())
			Expect(os.IsNotExist(err)).To(BeTrue())
		})

		Context("when a file cannot be removed", func() {
			It("returns an error", func() {
				extraRelease := cargo.CompiledRelease{
					Name:            "extra-release-that-cannot-be-deleted",
					Version:         "v0.0",
					StemcellOS:      "os-0",
					StemcellVersion: "v0.0.0",
				}

				extraReleases := map[cargo.CompiledRelease]string{}
				extraReleases[extraRelease] = "file-does-not-exist"

				err := localReleaseDirectory.DeleteExtraReleases(releasesDir, extraReleases, noConfirm)
				Expect(err).To(MatchError("failed to delete release extra-release-that-cannot-be-deleted"))
			})
		})
	})

	Describe("VerifyChecksums", func() {
		var (
			downloadedReleases map[cargo.CompiledRelease]string
			assetsLock         cargo.AssetsLock
			goodFilePath       string
			badFilePath        string
			err                error
		)

		BeforeEach(func() {
			goodFilePath = filepath.Join(releasesDir, "good-1.2.3-ubuntu-xenial-190.0.0.tgz")
			err = ioutil.WriteFile(goodFilePath, []byte("abc"), 0644)
			Expect(err).NotTo(HaveOccurred())

			badFilePath = filepath.Join(releasesDir, "bad-1.2.3-ubuntu-xenial-190.0.0.tgz")
			err = ioutil.WriteFile(badFilePath, []byte("some bad sha file"), 0644)
			Expect(err).NotTo(HaveOccurred())

			assetsLock = cargo.AssetsLock{
				Releases: []cargo.Release{
					{
						Name:    "good",
						Version: "1.2.3",
						SHA1:    "a9993e364706816aba3e25717850c26c9cd0d89d", // sha1 for string "abc"
					},
					{
						Name:    "bad",
						Version: "1.2.3",
						SHA1:    "a9993e364706816aba3e25717850c26c9cd0d89d", // sha1 for string "abc"
					},
				},
				Stemcell: cargo.Stemcell{
					OS:      "ubuntu-xenial",
					Version: "190.0.0",
				},
			}
		})

		Context("when all the checksums on the downloaded releases match their checksums in assets.lock", func() {
			It("succeeds", func() {
				downloadedReleases = map[cargo.CompiledRelease]string{
					{
						Name:            "good",
						Version:         "1.2.3",
						StemcellOS:      "ubuntu-xenial",
						StemcellVersion: "190.0.0",
					}: goodFilePath,
				}
				err := localReleaseDirectory.VerifyChecksums(releasesDir, downloadedReleases, assetsLock)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when at least one checksum on the downloaded releases does not match the checksum in assets.lock", func() {
			It("returns an error and deletes the bad release", func() {
				downloadedReleases = map[cargo.CompiledRelease]string{
					{
						Name:            "bad",
						Version:         "1.2.3",
						StemcellOS:      "ubuntu-xenial",
						StemcellVersion: "190.0.0",
					}: badFilePath,
				}
				err := localReleaseDirectory.VerifyChecksums(releasesDir, downloadedReleases, assetsLock)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("These downloaded releases do not match the checksum"))

				_, err = os.Stat(badFilePath)
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
		})
	})
})

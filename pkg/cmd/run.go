package cmd

import (
	"github.com/drausin/libri-experiments/pkg/sim"
	"github.com/drausin/libri/libri/librarian/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	durationFlag                = "duration"
	numAuthorsFlag              = "numAuthors"
	docsPerDayFlag              = "docsPerDay"
	contentSizeKBGammaShapeFlag = "contentSizeKBGammaShape"
	contentSizeKBGammaRateFlag  = "contentSizeKBGammaRate"
	sharesPerUploadFlag         = "sharesPerUpload"
	downloadWaitMinFlag         = "downloadWaitMin"
	downloadWaitMaxFlag         = "downloadWaitMax"
	nUploadersFlag              = "nUploaders"
	nDownloadersFlag            = "nDownloaders"
	librariansFlag              = "librarians"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "run an experiment",
	Long:  "run an experiment",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runExperiment()
	},
}

func init() {
	RootCmd.AddCommand(runCmd)

	runCmd.Flags().StringSliceP(librariansFlag, "a", nil,
		"comma-separated addresses (IPv4:Port) of librarian(s)")
	runCmd.Flags().Duration(durationFlag, sim.DefaultDuration,
		"experiment duration")
	runCmd.Flags().Uint(numAuthorsFlag, sim.DefaultNumAuthors,
		"number of authors (users)")
	runCmd.Flags().Uint(docsPerDayFlag, sim.DefaultDocsPerDay,
		"number docs an author uploads per day")
	runCmd.Flags().Float64(contentSizeKBGammaShapeFlag, sim.DefaultContentSizeKBGammaShape,
		"shape param of gamma distribution for content size (KBs)")
	runCmd.Flags().Float64(contentSizeKBGammaRateFlag, sim.DefaultContentSizeKBGammaRate,
		"rate param of gamma distribution for content size (KBs)")
	runCmd.Flags().Uint(sharesPerUploadFlag, sim.DefaultSharesPerUpload,
		"number of times each uploaded doc is shared")
	runCmd.Flags().Duration(downloadWaitMinFlag, sim.DefaultDownloadWaitMin,
		"lower bound of uniform distribution for wait time before downloading")
	runCmd.Flags().Duration(downloadWaitMaxFlag, sim.DefaultDownloadWaitMax,
		"upper bound of uniform distribution for wait time before downloading")
	runCmd.Flags().Uint(nUploadersFlag, sim.DefaultNUploaders,
		"number of uploader workers")
	runCmd.Flags().Uint(nDownloadersFlag, sim.DefaultNDownloaders,
		"number of downloader workers")
}

func runExperiment() error {
	librarianAddrs, err := server.ParseAddrs(viper.GetStringSlice(librariansFlag))
	if err != nil {
		return err
	}
	dataDir := viper.GetString(dataDirFlag)
	params := getParameters()
	runner := sim.NewRunner(params, dataDir, librarianAddrs)

	runner.Run()
	return nil
}

func getParameters() *sim.Parameters {
	return &sim.Parameters{
		Duration:                viper.GetDuration(durationFlag),
		NumAuthors:              uint(viper.GetInt(numAuthorsFlag)),
		DocsPerDay:              uint(viper.GetInt(docsPerDayFlag)),
		ContentSizeKBGammaShape: viper.GetFloat64(contentSizeKBGammaShapeFlag),
		ContentSizeKBGammaRate:  viper.GetFloat64(contentSizeKBGammaRateFlag),
		SharesPerUpload:         uint(viper.GetInt(sharesPerUploadFlag)),
		DownloadWaitMin:         viper.GetDuration(downloadWaitMinFlag),
		DownloadWaitMax:         viper.GetDuration(downloadWaitMaxFlag),
		NUploaders:              uint(viper.GetInt(nUploadersFlag)),
		NDownloaders:            uint(viper.GetInt(nDownloadersFlag)),
		LogLevel:                viper.GetString(logLevelFlag),
	}
}

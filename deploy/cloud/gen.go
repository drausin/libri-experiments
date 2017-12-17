package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/hashicorp/terraform/helper/variables"
	"github.com/spf13/cobra"
)

const (
	libriExpVersionVar         = "libri_exp_version"
	durationVar                = "duration"
	numAuthorsVar              = "num_authors"
	docsPerDayVar              = "docs_per_day"
	contentSizeKBGammaShapeVar = "content_size_kb_gamma_shape"
	contentSizeKBGammaRateVar  = "content_size_kb_gamma_rate"
	sharesPerUploadVar         = "shares_per_upload"
	numLibrariansVar           = "num_librarians"
	librarianLocalPortVar      = "librarian_local_port"
	numUploadesVar             = "num_uploaders"
	numDownloadersVar          = "num_downloaders"

	kubeTemplateDir            = "kubernetes"
	kubeConfigTemplateFilename = "libri-sim.template.yml"
	kubeConfigFilename         = "libri-sim.yml"
)

// SimConfig defines the simulation config params.
type SimConfig struct {
	LibriExpVersion         string
	Librarians              string
	Duration                string
	NumAuthors              uint
	DocsPerDay              uint
	ContentSizeKBGammaShape float64
	ContentSizeKBGammaRate  float64
	SharesPerUpload         uint
	NumUploaders            uint
	NumDownloaders          uint
}

var (
	expDefFilepath string
	outDir         string
)

var createCmd = &cobra.Command{
	Short: "create experiment trial Pod config",
	Long:  "create experiment trial Pod config",
	RunE: func(cmd *cobra.Command, args []string) error {
		return writeSimConfig(expDefFilepath, outDir)
	},
}

func main() {
	if err := createCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	createCmd.Flags().StringVarP(&expDefFilepath, "expDefFilepath", "e", "",
		"experiment definition *.tfvars file")
	createCmd.Flags().StringVarP(&outDir, "outDir", "d", "",
		fmt.Sprintf("output directory for k8s config %s file", kubeConfigFilename))
}

func writeSimConfig(tfvarsFilepath string, outDir string) error {
	config, err := getSimConfig(tfvarsFilepath)
	if err != nil {
		return err
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	templateFilename := filepath.Base(kubeConfigTemplateFilename)
	absTemplateFilepath := filepath.Join(wd, kubeTemplateDir, kubeConfigTemplateFilename)
	tmpl, err := template.New(templateFilename).ParseFiles(absTemplateFilepath)
	if err != nil {
		return err
	}

	kubeConfigFilepath := path.Join(outDir, kubeConfigFilename)
	out, err := os.Create(kubeConfigFilepath)
	if err != nil {
		return err
	}
	return tmpl.Execute(out, config)
}

func getSimConfig(tfvarsFilepath string) (*SimConfig, error) {
	tfvars := make(variables.FlagFile)
	if err := tfvars.Set(tfvarsFilepath); err != nil {
		return nil, err
	}
	numLibrarians := tfvars[numLibrariansVar].(int)
	librariansAddrs := make([]string, numLibrarians)
	librarianLocalPort := tfvars[librarianLocalPortVar].(int)
	for i := 0; i < numLibrarians; i++ {
		librariansAddrs[i] =
			fmt.Sprintf("librarians-%d.libri.default.svc.cluster.local:%d", i, librarianLocalPort)
	}
	config := &SimConfig{
		LibriExpVersion:         tfvars[libriExpVersionVar].(string),
		Librarians:              strings.Join(librariansAddrs, ","),
		Duration:                tfvars[durationVar].(string),
		NumAuthors:              uint(tfvars[numAuthorsVar].(int)),
		DocsPerDay:              uint(tfvars[docsPerDayVar].(int)),
		ContentSizeKBGammaShape: tfvars[contentSizeKBGammaShapeVar].(float64),
		ContentSizeKBGammaRate:  tfvars[contentSizeKBGammaRateVar].(float64),
		SharesPerUpload:         uint(tfvars[sharesPerUploadVar].(int)),
		NumUploaders:            uint(tfvars[numUploadesVar].(int)),
		NumDownloaders:          uint(tfvars[numDownloadersVar].(int)),
	}
	return config, nil
}

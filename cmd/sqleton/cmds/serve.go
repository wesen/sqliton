package cmds

import (
	"context"
	"embed"
	"fmt"
	"github.com/go-go-golems/clay/pkg/repositories"
	"github.com/go-go-golems/clay/pkg/watcher"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/alias"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/loaders"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/helpers"
	parka "github.com/go-go-golems/parka/pkg"
	"github.com/go-go-golems/parka/pkg/render"
	"github.com/go-go-golems/sqleton/pkg"
	"github.com/go-go-golems/sqleton/pkg/serve"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

type ServeCommand struct {
	description         *cmds.CommandDescription
	dbConnectionFactory pkg.DBConnectionFactory
	repositories        []string
	commands            []cmds.Command
	aliases             []*alias.CommandAlias
}

func (s *ServeCommand) Description() *cmds.CommandDescription {
	return s.description
}

//go:embed templates
var embeddedFiles embed.FS

//go:embed static
var staticFiles embed.FS

func (s *ServeCommand) Run(
	ctx context.Context,
	parsedLayers map[string]*layers.ParsedParameterLayer,
	ps map[string]interface{},
) error {
	// set up repository watcher
	r := repositories.NewRepository(
		repositories.WithDirectories(s.repositories),
		repositories.WithUpdateCallback(func(cmd cmds.Command) error {
			description := cmd.Description()
			log.Info().Str("name", description.Name).
				Str("source", description.Source).
				Msg("Updating cmd")
			// TODO(manuel, 2023-04-19) This is where we would recompute the HandlerFunc used below in GET and POST
			return nil
		}),
		repositories.WithRemoveCallback(func(cmd cmds.Command) error {
			description := cmd.Description()
			log.Info().Str("name", description.Name).
				Str("source", description.Source).
				Msg("Removing cmd")
			// TODO(manuel, 2023-04-19) This is where we would recompute the HandlerFunc used below in GET and POST
			return nil
		}),
	)

	for _, command := range s.commands {
		r.Add(command)
	}
	for _, alias := range s.aliases {
		r.Add(alias)
	}

	yamlLoader := &loaders.YAMLReaderCommandLoader{
		YAMLCommandLoader: &pkg.SqlCommandLoader{
			DBConnectionFactory: pkg.OpenDatabaseFromSqletonConnectionLayer,
		},
	}

	// now set up parka server
	port := ps["serve-port"].(int)
	host := ps["serve-host"].(string)
	dev, _ := ps["dev"].(bool)

	// TODO(manuel, 2023-04-19) These are currently handled as template dirs only, not as static dirs
	contentDirs := ps["content-dirs"].([]string)

	serverOptions := []parka.ServerOption{
		parka.WithPort(uint16(port)),
		parka.WithAddress(host),
		parka.WithGzip(),
	}

	if len(contentDirs) > 0 {
		lookups := make([]render.TemplateLookup, len(contentDirs))
		for i, contentDir := range contentDirs {
			isAbsoluteDir := strings.HasPrefix(contentDir, "/")

			// resolve base dir
			if !isAbsoluteDir {
				baseDir, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get working directory: %w", err)
				}
				contentDir = baseDir + "/" + contentDir
			}

			localTemplateLookup, err := render.LookupTemplateFromFSReloadable(
				os.DirFS(contentDir),
				"",
				"**/*.tmpl.md",
				"**/*.md",
				"**/*.tmpl.html",
				"**/*.html")
			if err != nil {
				return fmt.Errorf("failed to load local template: %w", err)
			}
			lookups[i] = localTemplateLookup
		}
		serverOptions = append(serverOptions, parka.WithAppendTemplateLookups(lookups...))
	}

	if dev {
		templateLookup, err := render.LookupTemplateFromFSReloadable(
			os.DirFS("."),
			"cmd/sqleton/cmds/templates/static",
			"**/*.tmpl.*",
		)
		if err != nil {
			return fmt.Errorf("failed to load local template: %w", err)
		}
		serverOptions = append(serverOptions,
			parka.WithAppendTemplateLookups(templateLookup),
			parka.WithStaticPaths(
				parka.NewStaticPath(http.FS(os.DirFS("cmd/sqleton/cmds/static")), "/static"),
			))
	} else {
		embeddedTemplateLookup, err := render.LookupTemplateFromFS(embeddedFiles, "templates", "**/*.tmpl.*")
		if err != nil {
			return fmt.Errorf("failed to load embedded template: %w", err)
		}
		serverOptions = append(serverOptions,
			parka.WithAppendTemplateLookups(embeddedTemplateLookup),
			parka.WithStaticPaths(
				parka.NewStaticPath(http.FS(parka.NewAddPrefixPathFS(staticFiles, "static/")), "/static"),
			),
		)
	}

	serverOptions = append(serverOptions,
		parka.WithDefaultParkaLookup(),
		parka.WithDefaultParkaStaticPaths(),
	)

	server, err := parka.NewServer(serverOptions...)
	if err != nil {
		return err
	}

	//glazedParameterLayers, err := cli.NewGlazedParameterLayers()
	if err != nil {
		return err
	}

	server.Router.StaticFileFS(
		"favicon.ico",
		"templates/favicon.ico",
		http.FS(embeddedFiles),
	)

	sqletonConnectionLayer := parsedLayers["sqleton-connection"]
	dbtConnectionLayer := parsedLayers["dbt"]

	// "cmd/sqleton/cmds/templates",

	cd := &serve.CommandDir{
		Repositories:      s.repositories,
		TemplateDirectory: "",
		TemplateName:      "",
		IndexTemplateName: "",
		AdditionalData:    nil,
		Defaults:          nil,
		Overrides:         nil,
	}

	var defaultTemplateFS fs.FS = embeddedFiles
	defaultTemplateDirectory := "templates"

	if dev {
		defaultTemplateFS = os.DirFS(".")
		defaultTemplateDirectory = "cmd/sqleton/cmds/templates"
	}

	cdOptions := &serve.ServeOptions{
		DevMode:                  dev,
		DefaultTemplateName:      "data-tables.tmpl.html",
		DefaultIndexTemplateName: "",
		DefaultTemplateFS:        defaultTemplateFS,
		DefaultTemplateDirectory: defaultTemplateDirectory,
		DbtConnectionLayer:       dbtConnectionLayer,
		SqletonConnectionLayer:   sqletonConnectionLayer,
	}

	err = cd.Serve(server, cdOptions, "")
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errGroup, ctx := errgroup.WithContext(ctx)
	errGroup.Go(func() error {
		return r.Watch(ctx, yamlLoader, nil,
			watcher.WithMask("**/*.yaml"),
		)
	})
	errGroup.Go(func() error {
		return server.Run(ctx)
	})
	errGroup.Go(func() error {
		return helpers.CancelOnSignal(ctx, os.Interrupt, cancel)
	})

	err = errGroup.Wait()
	if err != nil {
		return err
	}

	return nil
}

func NewServeCommand(
	dbConnectionFactory pkg.DBConnectionFactory,
	repositories []string, commands []cmds.Command, aliases []*alias.CommandAlias,
	options ...cmds.CommandDescriptionOption,
) *ServeCommand {
	options_ := append(options,
		cmds.WithShort("Serve the API"),
		cmds.WithArguments(),
		cmds.WithFlags(
			parameters.NewParameterDefinition(
				"serve-port",
				parameters.ParameterTypeInteger,
				parameters.WithShortFlag("p"),
				parameters.WithHelp("Port to serve the API on"),
				parameters.WithDefault(8080),
			),
			parameters.NewParameterDefinition(
				"serve-host",
				parameters.ParameterTypeString,
				parameters.WithHelp("Host to serve the API on"),
				parameters.WithDefault("localhost"),
			),
			parameters.NewParameterDefinition(
				"dev",
				parameters.ParameterTypeBool,
				parameters.WithHelp("Run in development mode"),
				parameters.WithDefault(false),
			),
			parameters.NewParameterDefinition(
				"content-dirs",
				parameters.ParameterTypeStringList,
				parameters.WithHelp("Serve static and templated files from these directories"),
				parameters.WithDefault([]string{}),
			),
		),
	)
	return &ServeCommand{
		dbConnectionFactory: dbConnectionFactory,
		description: cmds.NewCommandDescription(
			"serve",
			options_...,
		),
		repositories: repositories,
		commands:     commands,
		aliases:      aliases,
	}
}

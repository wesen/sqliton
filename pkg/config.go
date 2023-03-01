package pkg

import (
	"fmt"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type DatabaseConfig struct {
	Host            string `glazed.parameter:"host"`
	Database        string `glazed.parameter:"database"`
	User            string `glazed.parameter:"user"`
	Password        string `glazed.parameter:"password"`
	Port            int    `glazed.parameter:"port"`
	Schema          string `glazed.parameter:"schema"`
	Type            string `glazed.parameter:"db-type"`
	DSN             string `glazed.parameter:"dsn"`
	Driver          string `glazed.parameter:"driver"`
	DbtProfilesPath string `glazed.parameter:"dbt-profiles-path"`
	DbtProfile      string `glazed.parameter:"dbt-profile"`
	UseDbtProfiles  bool   `glazed.parameter:"use-dbt-profiles"`
}

// LogVerbose just outputs information about the database config to the
// debug logging level.
func (c *DatabaseConfig) LogVerbose() {
	if c.UseDbtProfiles {
		log.Debug().
			Str("dbt-profiles-path", c.DbtProfilesPath).
			Str("dbt-profile", c.DbtProfile).
			Msg("Using dbt profiles")

		log.Debug().
			Str("host", c.Host).
			Str("database", c.Database).
			Str("user", c.User).
			Int("port", c.Port).
			Str("schema", c.Schema).
			Str("type", c.Type).
			Msg("Using connection string")
	} else if c.DSN != "" {
		log.Debug().
			Str("dsn", c.DSN).
			Str("driver", c.Driver).
			Msg("Using DSN")
	} else {
		log.Debug().
			Str("host", c.Host).
			Str("database", c.Database).
			Str("user", c.User).
			Int("port", c.Port).
			Str("schema", c.Schema).
			Str("type", c.Type).
			Msg("Using connection string")
	}
}

func (c *DatabaseConfig) ToString() string {
	if c.UseDbtProfiles {
		s, err := c.GetSource()
		if err != nil {
			return fmt.Sprintf("Error: %s", err)
		}
		sourceString := fmt.Sprintf("%s@%s:%d/%s", s.Username, s.Hostname, s.Port, s.Database)

		if c.DbtProfilesPath != "" {
			return fmt.Sprintf("dbt-profiles-path: %s, dbt-profile: %s, %s", c.DbtProfilesPath, c.DbtProfile, sourceString)
		} else {
			return fmt.Sprintf("dbt-profile: %s, %s", c.DbtProfile, sourceString)
		}
	} else if c.DSN != "" {
		return fmt.Sprintf("dsn: %s, driver: %s", c.DSN, c.Driver)
	} else {
		return fmt.Sprintf("%s@%s:%d/%s", c.User, c.Host, c.Port, c.Database)
	}
}

func (c *DatabaseConfig) GetSource() (*Source, error) {
	// TODO(2022-12-18, manuel) This is where we would add support for DSN/Driver loading
	// See https://github.com/wesen/sqleton/issues/21
	_ = c.DSN
	_ = c.Driver

	var source *Source

	if c.UseDbtProfiles {
		sources, err := ParseDbtProfiles(c.DbtProfilesPath)
		if err != nil {
			return nil, err
		}

		//

		for _, s := range sources {
			if s.Name == c.DbtProfile {
				source = s
				break
			}
		}

		if source == nil {
			return nil, errors.Errorf("Source %s not found", c.DbtProfile)
		}
	} else {
		source = &Source{
			Type:     c.Type,
			Hostname: c.Host,
			Port:     c.Port,
			Username: c.User,
			Password: c.Password,
			Database: c.Database,
			Schema:   c.Schema,
		}
	}

	return source, nil
}

func (c *DatabaseConfig) Connect() (*sqlx.DB, error) {
	c.LogVerbose()

	s, err := c.GetSource()
	if err != nil {
		return nil, err
	}
	db, err := sqlx.Connect(s.Type, s.ToConnectionString())

	// TODO(2022-12-18, manuel): this is where we would add support for a ro connection
	// https://github.com/wesen/sqleton/issues/24

	return db, err
}

func OpenDatabaseFromSqletonConnectionLayer(parsedLayers map[string]*layers.ParsedParameterLayer) (*sqlx.DB, error) {
	config, err2 := NewConfigFromParsedLayers(parsedLayers)
	if err2 != nil {
		return nil, err2
	}
	return config.Connect()
}

func NewConfigFromParsedLayers(parsedLayers map[string]*layers.ParsedParameterLayer) (*DatabaseConfig, error) {
	sqletonConnectionLayer, ok := parsedLayers["sqleton-connection"]
	if !ok {
		return nil, errors.New("No sqleton-connection layer found")
	}
	config := &DatabaseConfig{}
	err := parameters.InitializeStructFromParameters(config, sqletonConnectionLayer.Parameters)
	if err != nil {
		return nil, err
	}
	return config, nil
}
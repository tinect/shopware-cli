package project

import (
	"database/sql"
	"io"
	"os"
	"strings"

	"github.com/FriendsOfShopware/shopware-cli/shop"

	"github.com/doutorfinancas/go-mad/core"
	"github.com/doutorfinancas/go-mad/database"
	"github.com/doutorfinancas/go-mad/generator"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var projectDatabaseDumpCmd = &cobra.Command{
	Use:   "dump [database]",
	Short: "Dumps the Shopware database",
	Args:  cobra.ExactArgs(1),
	RunE: func(cobraCmd *cobra.Command, args []string) error {
		host, _ := cobraCmd.Flags().GetString("host")
		port, _ := cobraCmd.Flags().GetString("port")
		username, _ := cobraCmd.Flags().GetString("username")
		password, _ := cobraCmd.Flags().GetString("password")
		output, _ := cobraCmd.Flags().GetString("output")
		clean, _ := cobraCmd.Flags().GetBool("clean")
		skipLockTables, _ := cobraCmd.Flags().GetBool("skip-lock-tables")
		anonymize, _ := cobraCmd.Flags().GetBool("anonymize")

		cfg := database.NewConfig(username, password, host, port, args[0])

		db, err := sql.Open("mysql", cfg.ConnectionString())

		if err != nil {
			return err
		}

		service := generator.NewService()
		var opt []database.Option
		opt = append(opt, database.OptionValue("hex-encode", "1"))
		opt = append(opt, database.OptionValue("set-charset", "utf8mb4"))
		opt = append(opt, database.OptionValue("dump-trigger", ""))
		opt = append(opt, database.OptionValue("skip-definer", ""))

		if skipLockTables {
			opt = append(opt, database.OptionValue("skip-lock-tables", "1"))
		}

		logger, _ := zap.NewProduction()
		dumper, err := database.NewMySQLDumper(db, logger, service, opt...)

		if err != nil {
			return err
		}

		pConf := core.Rules{Ignore: []string{}, NoData: []string{}, Where: map[string]string{}, Rewrite: map[string]core.Rewrite{}}

		if clean {
			pConf.NoData = append(pConf.NoData, "cart", "customer_recovery", "dead_message", "enqueue", "increment", "elasticsearch_index_task", "log_entry", "message_queue_stats", "notification", "payment_token", "refresh_token", "version", "version_commit", "version_commit_data", "webhook_event_log")
		}

		if anonymize {
			pConf.Rewrite = map[string]core.Rewrite{
				"customer": map[string]string{
					"first_name":     "faker.Person.FirstName()",
					"last_name":      "faker.Person.LastName()",
					"company":        "faker.Person.Name()",
					"title":          "faker.Person.Name()",
					"email":          "faker.Internet.Email()",
					"remote_address": "faker.Internet.Ipv4()",
				},
				"customer_address": map[string]string{
					"first_name":   "faker.Person.FirstName()",
					"last_name":    "faker.Person.LastName()",
					"company":      "faker.Person.Name()",
					"title":        "faker.Person.Name()",
					"street":       "faker.Address.StreetAddress()",
					"zipcode":      "faker.Address.PostCode()",
					"city":         "faker.Address.City()",
					"phone_number": "faker.Phone.Number()",
				},
				"log_entry": map[string]string{
					"provider": "",
				},
				"newsletter_recipient": map[string]string{
					"email":      "faker.Internet.Email()",
					"first_name": "faker.Person.FirstName()",
					"last_name":  "faker.Person.LastName()",
					"city":       "faker.Address.City()",
				},
				"order_address": map[string]string{
					"first_name":   "faker.Person.FirstName()",
					"last_name":    "faker.Person.LastName()",
					"company":      "faker.Person.Name()",
					"title":        "faker.Person.Name()",
					"street":       "faker.Address.StreetAddress()",
					"zipcode":      "faker.Address.PostCode()",
					"city":         "faker.Address.City()",
					"phone_number": "faker.Phone.Number()",
				},
				"order_customer": map[string]string{
					"first_name":     "faker.Person.FirstName()",
					"last_name":      "faker.Person.LastName()",
					"company":        "faker.Person.Name()",
					"title":          "faker.Person.Name()",
					"email":          "faker.Internet.Email()",
					"remote_address": "faker.Internet.Ipv4()",
				},
				"product_review": map[string]string{
					"email": "faker.Internet.Email()",
				},
				"user": map[string]string{
					"username":   "faker.Person.Name()",
					"first_name": "faker.Person.FirstName()",
					"last_name":  "faker.Person.LastName()",
					"email":      "faker.Internet.Email()",
				},
			}
		}

		var projectCfg *shop.Config
		if projectCfg, err = shop.ReadConfig(projectConfigPath); err != nil {
			if !strings.Contains(err.Error(), "cannot find .shopware-project.yml") {
				return err
			}
		}

		if projectCfg != nil && projectCfg.ConfigDump != nil {
			pConf.NoData = append(pConf.NoData, projectCfg.ConfigDump.NoData...)
			pConf.Ignore = append(pConf.Ignore, projectCfg.ConfigDump.Ignore...)
			for table, rewrites := range projectCfg.ConfigDump.Rewrite {
				_, ok := pConf.Rewrite[table]

				if !ok {
					pConf.Rewrite[table] = rewrites
				} else {
					for k, v := range rewrites {
						pConf.Rewrite[table][k] = v
					}
				}
			}
			pConf.Where = projectCfg.ConfigDump.Where
		}

		dumper.SetSelectMap(pConf.RewriteToMap())
		dumper.SetWhereMap(pConf.Where)
		if dErr := dumper.SetFilterMap(pConf.NoData, pConf.Ignore); dErr != nil {
			return dErr
		}

		var w io.Writer
		if w, err = os.Create(output); err != nil {
			return err
		}

		if err = dumper.Dump(w); err != nil {
			return err
		}

		log.Infof("Successfully created the dump %s", output)

		return nil
	},
}

func init() {
	projectRootCmd.AddCommand(projectDatabaseDumpCmd)
	projectDatabaseDumpCmd.Flags().String("host", "127.0.0.1", "hostname")
	projectDatabaseDumpCmd.Flags().String("username", "root", "mysql user")
	projectDatabaseDumpCmd.Flags().String("password", "root", "mysql password")
	projectDatabaseDumpCmd.Flags().String("port", "3306", "mysql port")
	projectDatabaseDumpCmd.Flags().String("output", "dump.sql", "file")
	projectDatabaseDumpCmd.Flags().Bool("clean", false, "Ignores cart, enqueue, message_queue_stats")
	projectDatabaseDumpCmd.Flags().Bool("skip-lock-tables", false, "Skips locking the tables")
	projectDatabaseDumpCmd.Flags().Bool("anonymize", false, "Anonymize customer data")
}

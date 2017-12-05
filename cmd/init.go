package cmd

import (
	"fmt"
	"os"

	"github.com/eiso/gpass/encrypt"
	"github.com/eiso/gpass/git"
	"github.com/eiso/gpass/utils"
	"github.com/spf13/cobra"
	"github.com/tucnak/store"
)

type InitCmd struct {
	key string
}

func NewInitCmd() *InitCmd {
	return &InitCmd{}
}

func (c *InitCmd) Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init /path/to/git-repository",
		Short: "Initializes gpass for a git repo.",
		Args:  cobra.ExactArgs(1),
		RunE:  c.Execute,
	}

	cmd.Flags().StringVarP(&c.key, "key", "k", "", "Path to your local private key.")
	cmd.MarkFlagRequired("key")

	return cmd
}

func (c *InitCmd) Execute(cmd *cobra.Command, args []string) error {

	u := new(git.User)
	r := new(git.Repository)

	if err := u.Init(); err != nil {
		return err
	}

	r.Path = args[0]

	if err := r.Load(); err != nil {
		return err
	}

	f, err := utils.LoadFile(c.key)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	k := encrypt.NewPGP(f, nil, nil, true)

	if err := k.Keyring(); err != nil {
		return fmt.Errorf("Unable to build keyring: %s", err)
	}

	Cfg.User = u
	Cfg.Repository = r
	Cfg.PrivateKey = c.key

	if err := store.Save("config.json", Cfg); err != nil {
		return fmt.Errorf("Failed to save the user config: %s", err)
	}

	fmt.Println("Successfully loaded your repository and private key\nConfig file written to your systems config folder as gpass/config.json")
	return nil
}

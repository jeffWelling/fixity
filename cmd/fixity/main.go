package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/jeffanddom/fixity/internal/auth"
	"github.com/jeffanddom/fixity/internal/config"
	"github.com/jeffanddom/fixity/internal/coordinator"
	"github.com/jeffanddom/fixity/internal/database"
	"github.com/jeffanddom/fixity/internal/migrate"
	"github.com/jeffanddom/fixity/internal/server"
)

var (
	version = "0.1.0-alpha"
	cfg     *config.Config
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "fixity",
		Short:   "Fixity - File Integrity & Lifecycle Monitoring",
		Version: version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var err error
			cfg, err = config.Load()
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}
			return nil
		},
	}

	rootCmd.AddCommand(serveCmd())
	rootCmd.AddCommand(userCmd())
	rootCmd.AddCommand(migrateCmd())
	rootCmd.AddCommand(versionCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Fixity HTTP server",
		Long: `Start the Fixity HTTP server with automatic database migration.

The server will:
1. Connect to the database
2. Automatically run any pending migrations
3. Start the HTTP server and coordinator
4. Handle graceful shutdown on SIGINT/SIGTERM`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Fixity v%s\n", version)
			fmt.Println("====================")

			// Connect to database
			fmt.Println("Connecting to database...")
			db, err := database.FromURL(cfg.Database.URL)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer db.Close()
			fmt.Println("✓ Database connected")

			// Auto-migrate database
			if err := migrate.AutoMigrate(db.DB(), "fixity"); err != nil {
				return fmt.Errorf("failed to run migrations: %w", err)
			}
			fmt.Println("✓ Database migrations complete")

			// Check for admin user
			users, err := db.Users.ListAll(context.Background())
			if err != nil {
				return fmt.Errorf("failed to check for users: %w", err)
			}

			if len(users) == 0 {
				fmt.Println("\n⚠️  WARNING: No users found in database!")
				fmt.Println("Create an admin user with: fixity user create --admin")
				fmt.Println()
			}

			// Create services
			authService := auth.NewService(db, auth.Config{})
			coord := coordinator.NewCoordinator(db, coordinator.Config{
				MaxConcurrentScans: cfg.Scanner.MaxConcurrentScans,
			})

			// Create server
			srv, err := server.New(db, authService, coord, server.Config{
				ListenAddr:        cfg.Server.ListenAddr,
				SessionCookieName: cfg.Server.SessionCookieName,
			})
			if err != nil {
				return fmt.Errorf("failed to create server: %w", err)
			}

			fmt.Println("✓ Services initialized")

			// Setup graceful shutdown
			shutdown := make(chan os.Signal, 1)
			signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

			// Start server in goroutine
			serverErrors := make(chan error, 1)
			go func() {
				fmt.Printf("\n🚀 Fixity server listening on %s\n", cfg.Server.ListenAddr)
				fmt.Println("Press Ctrl+C to stop")
				fmt.Println()
				serverErrors <- srv.Start()
			}()

			// Wait for shutdown signal or error
			select {
			case err := <-serverErrors:
				return fmt.Errorf("server error: %w", err)
			case sig := <-shutdown:
				fmt.Printf("\n\nReceived %v signal, shutting down gracefully...\n", sig)

				// Attempt graceful shutdown
				ctx, cancel := context.WithTimeout(context.Background(), 10)
				defer cancel()

				if err := srv.Shutdown(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "Error during shutdown: %v\n", err)
				}

				fmt.Println("✓ Server stopped")
				return nil
			}
		},
	}

	return cmd
}

func userCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "User management commands",
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new user",
		RunE: func(cmd *cobra.Command, args []string) error {
			username, _ := cmd.Flags().GetString("username")
			password, _ := cmd.Flags().GetString("password")
			email, _ := cmd.Flags().GetString("email")
			isAdmin, _ := cmd.Flags().GetBool("admin")

			if username == "" {
				return fmt.Errorf("username is required (--username)")
			}
			if password == "" {
				return fmt.Errorf("password is required (--password)")
			}

			// Connect to database
			db, err := database.FromURL(cfg.Database.URL)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer db.Close()

			// Run migrations first
			if err := migrate.AutoMigrate(db.DB(), "fixity"); err != nil {
				return fmt.Errorf("failed to run migrations: %w", err)
			}

			// Create auth service
			authService := auth.NewService(db, auth.Config{})

			// Create user
			user, err := authService.CreateUser(context.Background(), username, password, email, isAdmin)
			if err != nil {
				return fmt.Errorf("failed to create user: %w", err)
			}

			fmt.Printf("✓ User created successfully\n")
			fmt.Printf("  ID: %d\n", user.ID)
			fmt.Printf("  Username: %s\n", user.Username)
			if user.Email != nil {
				fmt.Printf("  Email: %s\n", *user.Email)
			}
			fmt.Printf("  Admin: %v\n", user.IsAdmin)
			fmt.Printf("  Created: %s\n", user.CreatedAt.Format("2006-01-02 15:04:05"))

			return nil
		},
	}

	createCmd.Flags().String("username", "", "Username (required)")
	createCmd.Flags().String("password", "", "Password (required)")
	createCmd.Flags().String("email", "", "Email address (optional)")
	createCmd.Flags().Bool("admin", false, "Make user an admin")

	cmd.AddCommand(createCmd)
	return cmd
}

func migrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Database migration commands",
	}

	upCmd := &cobra.Command{
		Use:   "up",
		Short: "Run all pending migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := database.FromURL(cfg.Database.URL)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer db.Close()

			return migrate.AutoMigrate(db.DB(), "fixity")
		},
	}

	downCmd := &cobra.Command{
		Use:   "down",
		Short: "Roll back the last migration",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := database.FromURL(cfg.Database.URL)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer db.Close()

			return migrate.MigrateDown(db.DB(), "fixity")
		},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List available migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			migrations, err := migrate.ListMigrations()
			if err != nil {
				return err
			}

			fmt.Println("Available migrations:")
			for _, m := range migrations {
				fmt.Printf("  - %s\n", m)
			}
			return nil
		},
	}

	cmd.AddCommand(upCmd, downCmd, listCmd)
	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Fixity v%s\n", version)
		},
	}
}

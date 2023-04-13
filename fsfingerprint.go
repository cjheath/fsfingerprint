/*
 * Walk specified file tree(s) and save/update the directory structure, file sizes and SHA256 checksums into SQLite
 *
 * REVISIT:: Update does not delete records of things that no longer exist
 */
package main

import (
	"os"						// Environment variables and exit
	"io"
	"log"						// Error logging and exit
	"fmt"						// formated I/O
	"strings"					// Split strings, etc
	"database/sql"					// Generic SQL interface
	hex "encoding/hex"
	_ "github.com/mattn/go-sqlite3"			// Driver for database/sql
	fs "io/fs"					// FileSystem
	sha "crypto/sha256"
)

var (
	sqlite		*sql.DB
	sqlinsert	*sql.Stmt
)

func sqlite_init() {
	var err	error
	fmt.Printf("Opening db %s\n", sqlite_file);
	sqlite, err = sql.Open("sqlite3", sqlite_file)
	if err != nil {
		log.Fatal(err);
	}

	schema := `
		CREATE TABLE IF NOT EXISTS file (
			id	INTEGER NOT NULL PRIMARY KEY,
			parent	INTEGER NULL,
			size	INTEGER NULL,
			sha256	VARCHAR NULL,
			name	INTEGER,
			error	VARCHAR NULL
		);
		CREATE UNIQUE INDEX IF NOT EXISTS file_index ON file (parent, name);
		CREATE INDEX IF NOT EXISTS sha_index ON file (sha256);
	`
	_, err = sqlite.Exec(schema);
	if err != nil {
		log.Fatal(err);
	}

	/*
		Can't use RETURNING clause here, invalid syntax:
		INSERT INTO file(parent, name, sha256) VALUES (?1, ?2, ?3)
		ON CONFLICT(parent, name) DO UPDATE SET sha256 = ?3 RETURNING id
	*/
	sqlinsert, err = sqlite.Prepare(`
		INSERT INTO file(parent, name, sha256, size, error) VALUES (?1, ?2, ?3, ?4, ?5)
			ON CONFLICT(parent, name) DO UPDATE SET sha256 = ?3, size = ?4, error = ?5
	`);
	if err != nil {
		log.Fatal(err);
	}
}

func update_or_add_file(parent int64, name string, sha256 *string, size *int64, fserr *string) int64 {
	_, err := sqlinsert.Exec(parent, name, sha256, size, fserr);
	if err != nil {
		fmt.Printf("UPSERT failed");
		log.Fatal(err);
	}

	rows, err := sqlite.Query("SELECT id FROM file WHERE parent = ? AND name = ?", parent, name)
	if err != nil {
		fmt.Printf("SELECT id failed:");
		log.Fatal(err);
	}
	if !rows.Next() {
		return 0
	}

	var id int64
	err = rows.Scan(&id);
	if err != nil {
		fmt.Printf("Scan failed:");
		log.Fatal(err);
	}
	defer rows.Close()

	return id
}

func scan_root(root string) {
	var parent_id	int64
	var fsys	fs.FS

	// Find or build the starting point in the database
	if root[0:1] == "/" {
		fsys = os.DirFS("/");
		parent_id = 0;
		root = root[1:]
	} else {
		cwd, _ := os.Getwd()
		cwpath := strings.Split(cwd, "/")

		/*
		 * Descend the tree from the filesystem root to the working directory,
		 * finding or adding directory entries as needed
		 */
		for _, path := range cwpath {
			parent_id = update_or_add_file(parent_id, path, nil, nil, nil)
		}
		fsys = os.DirFS(".");
	}

	// Descend the database tree to the root point
	root_path := strings.Split(root, "/")
	for _, path := range root_path {
		if path != "." {
			parent_id = update_or_add_file(parent_id, path, nil, nil, nil)
		}
	}

	scan_dir(fsys, parent_id, root);
}

func calc_sha256(fsys fs.FS, file_path string) (*[]byte, error) {
	file, err := fsys.Open(file_path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	hash := sha.New()
	if _, err = io.Copy(hash, file); err != nil {
		return nil, err
	}
	result := hash.Sum(nil)
	return &result, nil
}

func scan_dir(fsys fs.FS, parent_id int64, dir_path string) {
	// REVISIT: Calculate composite hash for entire directory
	entries, err := fs.ReadDir(fsys, dir_path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't read directory %s: %v\n", dir_path, err);
		return;
	}
	for _, entry := range entries {
		dir_prefix := ""
		if dir_path != "." {
			dir_prefix = dir_path+"/"
		}
		entry_path := dir_prefix + entry.Name()

		file_info, err := entry.Info()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get FileInfo for %s: %v\n", entry_path, err);
			continue;
		}
		switch (file_info.Mode() & fs.ModeType) {
		case 0:
			var sha256 *string
			var fserr *string
			var sizep *int64

			sha_bin, err := calc_sha256(fsys, entry_path)
			if err != nil {
				value := err.Error()
				fserr = &value
				sizep = nil
			} else {
				value := hex.EncodeToString((*sha_bin))
				sha256 = &value
				fserr = nil
				size := file_info.Size()
				sizep = &size
			}

			update_or_add_file(parent_id, entry.Name(), sha256, sizep, fserr)

		case fs.ModeDir:
			new_parent_id := update_or_add_file(parent_id, entry.Name(), nil, nil, nil)
			scan_dir(fsys, new_parent_id, entry_path)

		default:
			continue;	// Symlinks, Named Pipes, sockets, devices, etc
		}
	}
	// REVISIT: Update directory hash and return it
}

func main() {
	process_configuration();

	sqlite_init();
	defer sqlite.Close()
	defer sqlinsert.Close()

	for _, root := range file_roots {
		fmt.Printf("Scanning root %s\n", root);
		scan_root(root);
	}
}

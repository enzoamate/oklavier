package db

type RegistryWorkspace struct {
	ID            string   `db:"id" json:"id"`
	Slug          string   `db:"slug" json:"slug"`
	Name          string   `db:"name" json:"name"`
	Description   *string  `db:"description" json:"description"`
	IconURL       string   `db:"icon_url" json:"icon_url"`
	DockerImage   string   `db:"docker_image" json:"docker_image"`
	Version       string   `db:"version" json:"version"`
	Category      string   `db:"category" json:"category"`
	DefaultCores  float64  `db:"default_cores" json:"default_cores"`
	DefaultMemory int64    `db:"default_memory" json:"default_memory"`
	DefaultSHM    string   `db:"default_shm" json:"default_shm"`
	Maintainer    *string  `db:"maintainer" json:"maintainer"`
	IsOfficial    bool     `db:"is_official" json:"is_official"`
	Installed     bool     `json:"installed"`
}

func (db *DB) GetRegistry(category string) ([]RegistryWorkspace, error) {
	var items []RegistryWorkspace
	query := `SELECT r.id, r.slug, r.name, r.description, COALESCE(r.icon_url,'') as icon_url,
		r.docker_image, r.version, r.category, r.default_cores, r.default_memory,
		COALESCE(r.default_shm,'512m') as default_shm, r.maintainer, r.is_official
		FROM workspace_registry r`
	if category != "" && category != "all" {
		query += " WHERE r.category = $1"
		query += " ORDER BY r.is_official DESC, r.name"
		err := db.Select(&items, query, category)
		return items, err
	}
	query += " ORDER BY r.is_official DESC, r.name"
	err := db.Select(&items, query)

	// Mark installed ones
	for i, item := range items {
		var count int
		db.Get(&count, "SELECT COUNT(*) FROM workspace WHERE registry_id = $1", item.ID)
		items[i].Installed = count > 0
	}

	return items, err
}

func (db *DB) GetRegistryCategories() ([]string, error) {
	var cats []string
	err := db.Select(&cats, "SELECT DISTINCT category FROM workspace_registry ORDER BY category")
	return cats, err
}

func (db *DB) GetRegistryItem(id string) (*RegistryWorkspace, error) {
	var item RegistryWorkspace
	err := db.Get(&item, `SELECT id, slug, name, description, COALESCE(icon_url,'') as icon_url,
		docker_image, version, category, default_cores, default_memory,
		COALESCE(default_shm,'512m') as default_shm, maintainer, is_official
		FROM workspace_registry WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	var count int
	db.Get(&count, "SELECT COUNT(*) FROM workspace WHERE registry_id = $1", item.ID)
	item.Installed = count > 0
	return &item, nil
}

func (db *DB) InstallFromRegistry(registryID string) error {
	item, err := db.GetRegistryItem(registryID)
	if err != nil {
		return err
	}
	fullImage := item.DockerImage + ":" + item.Version
	_, err = db.Exec(`INSERT INTO workspace (name, friendly_name, description, image_src, docker_image, cores, memory, shm_size, category, registry_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		item.Slug, item.Name, item.Description, item.IconURL, fullImage,
		item.DefaultCores, item.DefaultMemory, item.DefaultSHM, item.Category, item.ID)
	return err
}

func (db *DB) UninstallFromRegistry(registryID string) error {
	_, err := db.Exec("DELETE FROM workspace WHERE registry_id = $1", registryID)
	return err
}

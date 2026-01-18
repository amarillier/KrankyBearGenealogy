# KrankyBear Genealogy
<img width="256" height="256" alt="KrankyBear Genealogy" src="https://github.com/user-attachments/assets/5ecff2a8-99d8-40b2-8d08-fce46741228e" />


A modern, cross-platform genealogy application originally inspired by Personal Ancestral File (PAF) which was discontinued in 2013, built with Go and Fyne.

![Version](https://img.shields.io/badge/version-1.0.0-blue)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey)

## Overview

KrankyBear Genealogy is a fast, modern alternative to discontinued genealogy applications like PAF. Unlike legacy software with dated interfaces and sluggish performance, it delivers a responsive, contemporary experience that genealogy enthusiasts deserve. Built with Go and Fyne, it combines the proven features of classic genealogy software with modern UX design and cross-platform compatibility (Windows, macOS, Linux).

Currently in active development with regular feature additions, it already matches—and in many areas surpasses—the capabilities of established genealogy applications. Future plans include mobile companion apps for iOS and Android. 

## Key Features

### 📊 Three-View Interface
- **Family View**: PAF-style display with current person, parents, spouse(s), and children
- **Pedigree View**: Multi-generation ancestor chart (4+ generations)
- **Individual View**: Sortable table of all people with complete data

### 💾 Data Management
- **SQLite Database**: Reliable local storage with hot database switching
- **GEDCOM Import/Export**: Full compatibility with standard genealogy format
  - Export entire database or selected branch (person + descendants)
- **GenoPro Import**: Import .gno files (XML-based genealogy format)
- **Gramps Import**: Import Gramps .db databases with notes and media references
- **Backup/Restore**: Timestamped ZIP backups for easy data protection

### 👥 Relationship Management
- **Smart Linking**: Auto-link children to both parents when married
- **Multiple Marriages**: Track multiple spouses with marriage dates, places, and end reasons
- **Relationship Types**: Marriage, cohabitation, partner, or other informal relationships
- **Auto-Navigation**: Click any person to view their record in all views
- **Relationship Calculator**: Shows relationship to focus person (cousin, uncle, in-law, etc.)

### 📸 Media Management ✨ **NEW**
- **Photo & Document Storage**: Attach images, PDFs, videos, Office documents
- **Blob Storage**: Images stored in database for portability
- **External Linking**: Optional external file references for large media
- **Many-to-Many Links**: Link one photo to multiple people (family photos, weddings)
- **Media Library**: Centralized view and management of all media
- **Supported Formats**: 
  - Images: JPEG, PNG, GIF, WebP
  - Videos: MP4, MOV
  - Documents: PDF, Word (.doc/.docx), Excel (.xls/.xlsx), PowerPoint (.ppt/.pptx)
- **Thumbnail Generation**: Fast loading with automatic thumbnails
- **Broken Link Detection**: Visual warnings for missing external files

### 📋 Contact Information
- Address, City, State/Province, Postal Code, Country
- Email and Phone
- Searchable and exportable

### 📊 Reports
- **Data Quality Report**: Interactive report showing incomplete records
- **Living Status Report**: Identifies inconsistencies based on birth date and living status
- Clickable reports - navigate directly to any person

### ⚙️ User Experience
- **Focus User Preference**: Designate a home person for startup
- **Alphabetical Sorting**: Left panel sorted by surname then given name
- **Context Switching**: Selected person syncs across all three views
- **Double-Click to Edit**: Quick access to person editor
- **Search/Filter**: Universal filter works across all views
- **Theme Support**: Light, Dark, or System theme
- **System Tray Integration**: Menu bar and system tray menus (Hide/Show, About, Help, Check for Update, Quit)
- **Auto Update Checker**: Notifies when new versions are available

## Quick Start

### Installation

Download the latest release for your platform from the [Releases](https://github.com/amarillier/KrankyBearGenealogy/releases) page.

### First Run

1. Launch the application
2. The app creates an empty database on first run
3. Choose to:
   - **Import existing data** (GEDCOM, GenoPro, or Gramps)
   - **Start fresh** by adding your first person

### Building from Source

```bash
# Prerequisites: Go 1.21+
git clone https://github.com/amarillier/KrankyBearGenealogy.git
cd KrankyBearGenealogy

# Build
go build -mod=mod -ldflags="-s -w" -trimpath -o genealogy .

## Usage

### Main Toolbar
- **Add Person**: Create a new person record
- **Delete Person**: Remove selected person and relationships
- **Focus Person**: Navigate to your designated "home" person
- **Add Media**: Attach photos/documents to multiple people
- **Media Library**: Browse and manage all media

### Menus
- **File**: New Database, Open Database, Import/Export, Backup/Restore
- **Reports**: Data Quality Report, Living Status Report
- **Media**: Media Library, Add Media
- **Settings**: Configure focus user and preferences
- **Help**: About, Help, Check for Update

### Working with People
- **Filter/Search**: Type in left panel to filter by name
- **Navigate**: Click any person's name to view their record
- **Edit**: Double-click name or use "Edit Individual" button
- **Add Relationships**: Use "Add Parent", "Add Spouse", "Add Child" buttons
  - Choose to link existing person or create new
  - Searchable selection dialogs for existing people
  - Auto-link children to both parents when applicable

### Managing Media
1. **Per-Person**: Edit person → "Manage Photos & Media"
2. **Global**: Use "Add Media" button to link to multiple people
3. **Media Library**: Browse all media, filter, edit metadata, manage links
4. **Storage Options**: Store in database (default) or link to external file

## Date Formats Supported

- **Full Dates**: "3 DEC 1931", "1931-12-03"
- **Partial Dates**: "1931", "DEC 1931"
- **Approximate**: "abt 1945", "bet 1940 and 1950"

## Documentation

See the `docs/` folder for additional documentation:
- [ROADMAP.md](docs/ROADMAP.md) - Feature roadmap and future plans
- [FEATURES.md](docs/FEATURES.md) - Detailed feature list
- [CHANGELOG.md](docs/CHANGELOG.md) - Version history
- [GENOPRO_IMPORT.md](docs/GENOPRO_IMPORT.md) - GenoPro import details

## What Makes KrankyBear Different?

### Compared to PAF (Personal Ancestral File)
✅ **Modern & Cross-Platform**: Runs on macOS, Windows, Linux (PAF was Windows-only)  
✅ **Active Development**: PAF was discontinued in 2013  
✅ **Better Navigation**: Three synchronized views with instant context switching  
✅ **Media Management**: Built-in photo and document support  
✅ **Smart Features**: Auto-linking, relationship calculator, data quality reports  

### Compared to Gramps
✅ **Faster**: Optimized UI, instant view switching  
✅ **Simpler**: Cleaner interface, easier learning curve  
✅ **Better UX**: Modern design with intuitive navigation  
✅ **Portability**: Single database file with embedded media  

### Compared to GenoPro
✅ **Lightweight**: Fast startup, low memory usage  
✅ **Data-Focused**: Emphasizes data management over complex visualizations  
✅ **Open Source**: Extensible and customizable  

## Performance

- **App Launch**: < 2 seconds
- **Database Load** (1000+ people): < 1 second
- **View Switching**: < 200ms
- **Search**: Real-time filtering

## Database Structure

The SQLite database includes:
- `persons` - Individual records
- `relationships` - Parent-child, spouse relationships
- `marriages` - Marriage/relationship details
- `media` - Photos, documents, videos
- `person_media` - Many-to-many links between people and media
- `events` - Life events (future expansion)
- `identifiers` - External IDs and UIDs
- `sources` - Citations (future expansion)

## Roadmap

**Completed (v1.0)**
- ✅ Three-view interface
- ✅ GEDCOM, GenoPro, Gramps import
- ✅ Media management (photos, documents, videos)
- ✅ Multiple marriage support
- ✅ Relationship calculator
- ✅ Data quality reports
- ✅ Backup/restore

**Coming Soon (v1.1+)**
- 📊 Additional reports (timeline, statistics, duplicates)
- 🔍 Research tools (research log, source citations)
- 🤝 GEDCOM merge and conflict resolution
- 💡 Keyboard shortcuts and advanced search
- 🎨 Enhanced visualizations (fan charts, maps)

See [ROADMAP.md](docs/ROADMAP.md) for complete future plans.

## Contributing

Contributions welcome! Please feel free to:
- Report bugs via [GitHub Issues](https://github.com/amarillier/KrankyBearGenealogy/issues)
- Submit feature requests
- Contribute code via pull requests

## License

See [LICENSE](LICENSE) file for details.

## Acknowledgments

- Inspired by Personal Ancestral File (PAF) from FamilySearch
- Built with [Fyne](https://fyne.io/) cross-platform UI framework
- Uses [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) for pure Go SQLite

## Support

- **Issues**: [GitHub Issues](https://github.com/amarillier/KrankyBearGenealogy/issues)
- **Updates**: Check for updates in-app or visit [Releases](https://github.com/amarillier/KrankyBearGenealogy/releases)

---

**⚠️ Important**: Always backup your database regularly! Use the built-in backup feature or keep exported GEDCOM files as backups.

*Built with ❤️ for genealogy enthusiasts*

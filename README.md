# KrankyBear Genealogy
<img width="256" height="256" alt="KrankyBear Genealogy" src="https://github.com/user-attachments/assets/5ecff2a8-99d8-40b2-8d08-fce46741228e" />


A modern, cross-platform genealogy application originally inspired by Personal Ancestral File (PAF) which was discontinued in 2013, built with Go and Fyne.

![Version](https://img.shields.io/badge/version-1.8.0-blue)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey)

## Overview

KrankyBear Genealogy is a fast, modern alternative to discontinued genealogy applications like PAF. Unlike legacy software with dated interfaces and sluggish performance, it delivers a responsive, contemporary experience that genealogy enthusiasts deserve. Built with Go and Fyne, it combines the proven features of classic genealogy software with modern UX design and cross-platform compatibility (Windows, macOS, Linux).

Currently in active development with regular feature additions, it already matches—and in many areas surpasses—the capabilities of established genealogy applications. Future plans include mobile companion apps for iOS and Android. 

- **🆓 100% Free** - Open source, no restrictions
- **🌍 Cross-Platform** - Windows, macOS, Linux

## Key Features

### 📊 Visualization Suite ✨ **NEW IN v1.3**
- **Family View**: PAF-style display with current person, parents, spouse(s), and children
- **Enhanced Pedigree View**: Multi-generation ancestor chart with advanced features
  - **Dynamic Generations**: Select 4, 5, 6, 7, or 8 generations
  - **Color Coding**: Color by gender, living status, or data completeness
  - **Zoom Controls**: Scale from 50% to 200% for better viewing
  - **Collapsible Branches**: Collapse ancestor lines to focus on specific lineages
  - **Visual Indicators**: Bookmarks (★), media (📷), sources (📚), todos (📝), research logs (🔍)
  - **Context Menus**: Right-click any person for quick actions
- **Fan Chart**: Circular ancestor visualization
  - **Beautiful Circular Layout**: Ancestors displayed in concentric rings
  - **3-6 Generations**: Selectable generation depth
  - **Gender-Based Colors**: Pink for females, blue for males
  - **Enhanced Mode**: Show spouses and children alongside ancestors
  - **Interactive Navigation**: Click any box to navigate to that person
  - **Dynamic Sync**: Automatically updates when navigating in Family View
  - **Context Menus**: Right-click for quick actions
- **Descendant Chart**: Visual tree showing children and future generations
  - **Dynamic Generations**: Select 3, 4, 5, 6, 7, or 8 generations
  - **Color Coding**: Same modes as Pedigree View (gender, living status, completeness)
  - **Zoom Controls**: Scale from 50% to 200%
  - **Enhanced Mode**: Show spouses grouped with their children
  - **Collapsible Branches**: Focus on specific descendant lines
  - **Dynamic Sync**: Bidirectional sync with Family View navigation
  - **Context Menus**: Right-click for quick actions
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

### 📋 Contact Information & Names
- **Preferred Name/Nickname Field**: Display "GivenName (Preferred) Surname" format for nicknames, middle names, or stage names
- Address, City, State/Province, Postal Code, Country
- Email and Phone
- Searchable and exportable

### 🔧 Database Maintenance ✨ **NEW IN v1.2**
- **Comprehensive Maintenance Tools**: Separate window with three tabs (Statistics, Maintenance, Analysis)
- **Statistics**: Real-time database size, record counts, and data quality issue detection
- **Maintenance Operations**:
  - Vacuum Database: Optimize and reclaim unused space
  - Check Integrity: Verify database is not corrupted
  - Remove Orphaned Media: Clean up unlinked media files
  - Remove Orphaned Relationships: Fix broken relationship links
  - Remove Duplicate Relationships: Clean up duplicate entries
- **Analysis Tools**:
  - Find Unused Sources: Identify sources with no citations
  - Optimize Query Performance: Update SQLite statistics
- **Keyboard Shortcut**: Cmd/Ctrl+L
- **User-Friendly**: Asynchronous operations, detailed reports, resizable window

### 🖱️ Quick Actions / Context Menus ✨ **NEW IN v1.3**
- **Right-Click Menus**: Available on all person boxes in every view
- **Navigation Shortcuts**: "Set as Focus Person", "View in Fan Chart", "View in Descendant Chart"
- **Quick Actions**: Add/Remove Bookmark, Add To-Do, View To-Dos
- **Sources & Research**: View Sources, Add Source, View Research Log
- **Copy Information**: Copy Name, Copy ID to clipboard
- **Generate Reports**: Ancestor Report, Descendant Report for any person
- **Universal Access**: Works in Family View, Pedigree View, Fan Chart, Descendant Chart

### 🛠️ Tools ✨ **NEW IN v1.3**
- **Advanced Search**: Multi-field search with export (Keyboard: Cmd/Ctrl+Shift+F)
  - Search by name, dates, places, and gender
  - Boolean filters (living/deceased, has media, sources, todos, bookmarked)
  - Export results to CSV, JSON, or XML
- **Relationship Calculator**: Calculate relationships between any two people
  - Detects direct relationships, cousins, in-laws
  - Shows common ancestors and line of descent
- **Global Search and Replace**: Batch text replacement across all records
  - Search in Birth/Death Place, Address, City, State, Country, Notes, or All Place Fields
  - Case-sensitive option
  - Preview all matches before applying
  - Perfect for standardizing place names (e.g., "USA" → "United States")
- **Global Name Case Conversion**: Fix inconsistent name capitalization
  - Convert to Proper Case, UPPERCASE, or lowercase
  - Smart handling of special cases (McDonald, O'Brien, van der Berg)
  - Roman numerals and suffixes handled properly
  - Preview all affected records before applying

### 🗺️ Map View & Geographic Analysis ✨ **NEW IN v1.5.1**
- **Interactive Map Visualization**: OpenStreetMap-based mapping of life events
  - **18 Zoom Levels**: From world view to street level
  - **Auto-Centering**: Automatically centers on person's birthplace
  - **Multi-Person Views**: View all people, descendants, or ancestors
  - **Event Filtering**: Toggle birth, death, and marriage markers
  - **Date Range Filter**: Focus on specific time periods
  - **Surname Filter**: View specific family lines geographically
  - **Generation Color-Coding**: Visualize family expansion over time
  - **Marker Statistics**: Real-time counts of visible events
  - **Center Buttons**: Quickly re-center on any event type
  - **Smart Place Handling**: Filters unknown/placeholder locations
  - **Geocoding Cache**: Fast map rendering with database caching
  - **Internet Check**: Gracefully handles offline scenarios
- **Geographic Reports**: Four specialized geographic analysis reports
  - **Migration Distance**: Rank families by how far they moved between generations
  - **Geographic Hotspots**: Identify concentration areas for births/deaths/marriages
  - **Cross-Border Families**: Find people with events in multiple countries
  - **Unmapped Places**: List places that need geocoding

### ↩️ Undo/Redo System ✨ **NEW IN v1.5.2**
- **Full Edit History**: Undo and redo all database operations
  - **Person Edits**: Revert changes to any person field
  - **Person Deletions**: Restore deleted people with all relationships intact
  - **Relationship Changes**: Undo/redo relationship additions and deletions
  - **50-Operation Buffer**: Maintains history of recent operations
- **Keyboard Shortcuts**: Cmd/Ctrl+U (Undo), Cmd/Ctrl+Shift+U (Redo)
- **Menu Integration**: Edit → Undo/Redo (also in system tray)
- **Thread-Safe**: Atomic operations with automatic UI updates
- **Safety Net**: Non-destructive, perfect for data entry confidence

### 📅 Enhanced Date Entry ✨ **NEW IN v1.5.3**
- **Smart Date Validation**: Real-time feedback with visual indicators
  - ✓ Green checkmark for valid dates
  - ⚠ Warning icon for invalid dates
  - Tooltips showing recognized format
- **Calendar Picker**: 📅 button on all date fields for easy date selection
- **Comprehensive Format Support**:
  - Standard: YYYY-MM-DD (e.g., 1945-05-08)
  - Genealogy: DD Mon YYYY (e.g., 8 May 1945)
  - Partial: Mon YYYY (e.g., May 1945) or YYYY (e.g., 1945)
  - US format: M/D/YYYY or MM/DD/YYYY
- **Genealogy Date Qualifiers**:
  - "abt 1945" or "about 1945" for estimated dates
  - "bef 1945" or "before 1945" for dates before
  - "aft 1945" or "after 1945" for dates after
  - "ca 1945" or "circa 1945" for approximate dates

### 📋 Alternate Names / Name Variations ✨ **NEW IN v1.5.3**
- **Track Name Variations**: Manage nicknames, maiden names, spelling variants
  - **Name Types**: Nickname, maiden, married, spelling, other
  - **Management UI**: Add, edit, delete alternate names
  - **Search Integration**: Find people by any name variation
  - **Display Integration**: "Also known as" shown in person cards
  - **Notes Field**: Document why names differ (e.g., "Used in 1900 census")
- **Use Cases**:
  - Track maiden names separately from married names
  - Document nickname usage (William → Bill)
  - Record spelling variations (Smith → Smyth)
  - Find people despite spelling differences in records

### 📝 Project Notes & Research Tools ✨ **NEW IN v1.5.1**
- **Project Notes / Scratchpad**: Database-wide notes not tied to specific people
- **People with Research Logs Report**: List all people with research activity
- **Research Log Manager**: Track all research activities with date/repository/results

### 📊 Reports
- **Data Quality Report**: Interactive report showing incomplete records
- **Living Status Report**: Identifies inconsistencies based on birth date and living status
- **Bulk Privacy Settings** ✨ **NEW IN v1.5.3**: Batch operations for privacy management (mark living/deceased, clear contact info)
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
   - **Load Demo Database** (Help → Load Demo Database) - Try out features with a sample family tree
   - **Import existing data** (GEDCOM, GenoPro, or Gramps)
   - **Start fresh** by adding your first person

### Demo Database

Want to explore features before entering your own data? Load the built-in demo database:
- **Help → Load Demo Database** to create a sample family tree
- **Almost 50 people** across 5-6 generations (1850s-2020s)
- Includes **preferred names/nicknames** (Mike, Bill, Beth, Chris)
- **Geographic diversity**: USA + international (Latvia)
- **Easter egg**: Stanley Yelnats family from "Holes" - four generations of the palindrome name!
- Demonstrates all features: multiple marriages, media, citations, research logs, bookmarks, and more
- Perfect for testing and learning the application

## Cross-platform support

- **Linux**: GNOME, KDE, XFCE, Cinnamon, MATE, etc. on X11 or Wayland.
- **macOS**: 10.13 (High Sierra) or later.
- **Windows**: Windows 10 or later. Some VMs and locked-down hosts have no
  usable hardware OpenGL, which most Fyne apps otherwise crash or hang on
  with no explanation — ProcessMiner automatically probes for it at launch and,
  only if that fails, falls back to a bundled Mesa3D software renderer and
  relaunches itself, with no user action needed. Real hardware OpenGL is
  always preferred when available (it's faster); nothing changes on a
  normal machine with a working GPU. The installer bundles this fallback
  automatically. The **portable (zip) Windows build does not** — if you're
  running the portable version on a machine without hardware OpenGL, grab
  `mesa-fallback.zip` from the same release, extract it into a
  `mesa-fallback` folder next to `KrankyBearProcessMiner.exe`, and the same
  automatic fallback applies. Most users on a normal machine will never
  need this file at all.

## Known limitations
None currently listed

### Building from Source

```bash
# Prerequisites: Go 1.21+
git clone https://github.com/amarillier/KrankyBearGenealogy.git
cd KrankyBearGenealogy

# Build
go build -mod=mod -ldflags="-s -w" -trimpath -o genealogy .
```

## Usage

### Main Toolbar
- **Add Person**: Create a new person record
- **Delete Person**: Remove selected person and relationships
- **Focus Person**: Navigate to your designated "home" person
- **Add Media**: Attach photos/documents to multiple people
- **Media Library**: Browse and manage all media

### Menus
- **File**: New Database, Open Database, Import/Export, Backup/Restore, Database Maintenance
- **Edit**: Undo, Redo (with keyboard shortcuts)
- **Media**: Media Library, Add Media, Sources Library, Research Log
- **Tools**: Advanced Search, Map View, Relationship Calculator, Date Calculator, Geocoding, Search & Replace, Name Case Conversion, Project Notes
- **Reports**: Statistics Dashboard, Data Quality, Living Status, Conflicts, Duplicates, Timeline, Geographic Reports, and more
- **Settings**: Configure focus user, preferences, keyboard shortcuts, themes
- **Help**: About, Help, Check for Update, Load Demo Database

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

## Date Formats Supported ✨ **Enhanced in v1.5.3**

KrankyBear now features **smart date validation** with real-time feedback and a calendar picker!

**Supported Formats:**
- **Full Dates**: "3 DEC 1931", "1931-12-03", "8 May 1945"
- **Partial Dates**: "1931", "DEC 1931", "May 1945"
- **Approximate**: "abt 1945", "ca 1945", "circa 1945"
- **Before/After**: "bef 1945", "aft 1945"
- **US Format**: "5/8/1945", "05/08/1945"

All date fields now show:
- ✓ Green checkmark for valid dates
- ⚠ Warning for invalid dates
- 📅 Calendar picker button for easy date selection

## Documentation

See the `docs/` folder for additional documentation:
- [ROADMAP.md](ROADMAP.md) - Feature roadmap and future plans
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
- `persons` - Individual records with full genealogy data
- `alternate_names` - Name variations (nicknames, maiden names, spellings)
- `relationships` - Parent-child, spouse relationships with marriage details
- `media` - Photos, documents, videos with BLOB or external storage
- `person_media` - Many-to-many links between people and media
- `sources` - Source citations with confidence levels
- `citations` - Links between sources and people
- `research_logs` - Research activity tracking
- `research_todos` - Per-person research tasks
- `validated_items` - Reviewed conflicts and duplicates
- `place_geocodes` - Geocoding cache for map view
- `project_notes` - Database-wide notes/scratchpad
- `events` - Life events
- `identifiers` - External IDs and UIDs

## Roadmap

**Completed (v1.0-v1.8)**
- ✅ Three-view interface (Family, Pedigree, Individual)
- ✅ GEDCOM, GenoPro, Gramps import/export
- ✅ Media management (photos, documents, videos)
- ✅ Multiple marriage support
- ✅ Relationship calculator (independent and focus-based)
- ✅ Comprehensive reports (data quality, conflicts, timeline, statistics, duplicates, geographic)
- ✅ Research tools (research log, source citations, to-do lists, project notes)
- ✅ Recent people tracking & bookmarks
- ✅ Keyboard shortcuts (30+ customizable shortcuts)
- ✅ Preferred name/nickname field
- ✅ Alternate names / name variations (v1.5.3)
- ✅ Backup/restore
- ✅ Database maintenance tools (vacuum, integrity check, orphan removal)
- ✅ Advanced search with multi-field filters and export
- ✅ Enhanced visualizations (fan charts, descendant charts, map view)
- ✅ Undo/Redo system (v1.5.2)
- ✅ Smart date entry with validation and calendar picker (v1.5.3)
- ✅ Geographic analysis (map view, migration reports, geocoding)
- ✅ Name/place autocomplete and authority helpers (v1.6-v1.7)
- ✅ Historical place mappings and background geocoding (v1.7)
- ✅ Full-text search with FTS5 and advanced query syntax (v1.8)
- ✅ Map export (PNG/PDF) and map interaction improvements (v1.8)

**Coming Soon**
- 🤝 GEDCOM merge and conflict resolution
- 🧾 Change tracking / audit log
- ⚡ Large-database optimizations (lazy loading and background processing)
- 🔐 Optional database encryption / password protection
- 📱 Mobile companion apps (iOS, Android)

See [ROADMAP.md](ROADMAP.md) for complete future plans.

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


## 💾 Other Data Management Notes
### Importing from Gramps (Old BSDDB Format)

If you have an old Gramps database in BSDDB format (pre-SQLite):

1. Install Gramps 3.x in a virtual machine or container
2. Open your old database in Gramps 3.x
3. Export to GEDCOM format (File → Export → GEDCOM)
4. Import the GEDCOM file into KrankyBear Genealogy

Note: Gramps themselves may have deprecated BSDDB format and be unable to
reliably convert it, or you may see database corruption error messages.
GEDCOM export from an old Gramps version is the standard migration path
recommended by the Gramps project.
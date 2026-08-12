# KrankyBear Genealogy - Feature Roadmap

## Vision
Build a modern, fast, cross-platform genealogy application that combines the best of PAF with contemporary UX, while remaining simpler and more intuitive than Gramps, GenoPro, and other legacy tools.

---

## ✅ Completed Features (v1.0)
- Cross-platform support (Mac, Windows, Linux)
- SQLite database storage
- GEDCOM import/export (with branch-specific export)
- GenoPro (.gno) import
- Gramps (.db) import (Phase 1 & 2: core data, notes, media references)
- Three-view interface (Family, Pedigree, Individual)
- Context switching between views
- Smart relationship linking (auto-link children to both parents)
- Multiple marriage support with PAF-style display
  - Smart prioritization: defaults to current/most recent marriage
  - Improved button layout: two-row design for better width management
- Marriage/relationship editing (dates, places, types, end reasons)
- Contact information fields (address, email, phone)
- **Media management (photos, documents) with blob storage**
- **Thumbnail generation for fast loading**
- **Media viewer/gallery per person**
- **Keyboard shortcuts** (23 shortcuts for common actions)
- **Statistics Dashboard** (comprehensive database statistics)
- Data Quality Report (interactive)
- Living Status Report (age-based inconsistency detection)
- Focus User preference with search
- System tray and menu bar integration
- Theme support (Light/Dark/System)
- Update checker integration
- Relationship calculator (shows relationship to focus user)
- **Independent Relationship Calculator** (calculate relationships between any two people)
- Database backup/restore (timestamped ZIP files)
- Alphabetical sorting with incomplete records at end
- Delete person with relationship cleanup
- Hot database switching (no restart required)

---

## ✅ Phase 2: Data Import & Migration (Completed)

### Gramps Import ⭐ HIGH PRIORITY
**Phase 1: Core Data (✓ Complete)**
- ✅ Import people (name, gender)
- ✅ Parse JSON to extract birth/death dates and places
- ✅ Import family relationships (spouse, parent-child)
- ✅ Map Gramps handles to internal IDs
- ✅ Deduplication based on name+birthdate

**Phase 2: Enhanced Data (✓ Complete)**
- ✅ Import notes to Person.Notes field
- ✅ Import UIDs for cross-reference
- ✅ Import media references (paths and descriptions) to notes


### Other Import Formats (Low Priority / Not Needed)
- ⏳ Most applications (Family Tree Maker, Ancestry.com, MyHeritage, etc.) export to GEDCOM
- ✅ Our robust GEDCOM import handles these already
- ✅ Only implemented GenoPro and Gramps because they have unique benefits:
  - ✅ GenoPro: Proprietary XML format users may not know how to export
  - ✅ Gramps: SQLite database preserves more structure and notes

---

## ✅ Phase 3: Media & Documents (Complete)

### Core Media Features ⭐ HIGH PRIORITY (✓ Complete)
**Default: Blob Storage in Database**
- ✅ Store images as BLOBs in database (single-file portability)
- ✅ Generate and store thumbnails for fast UI loading
- ✅ Store full-resolution images for viewing/editing
- ✅ Lazy-load full-res only when needed
- ✅ Supported image formats: JPEG, PNG, GIF, WebP

**Optional: External Media Links**
- ✅ Allow linking to external files for power users
- ✅ Clear UI indication when media is "external" (🔗 icon for working links)
- ✅ OS default viewer for external files
- ✅ Warning system for broken external links (⚠️ icon + "BROKEN LINK" message)

**Many-to-Many Media Linking** (✓ Complete)
- ✅ Link single media to multiple people (family photos, wedding photos)
- ✅ "Unlink" vs "Delete" logic (unlink removes link, delete only when last link)
- ✅ Show link count and linked people in media views
- ✅ Manage links dialog (add/remove people from media)

**Implementation Completed:**
1. ✅ `media` table with many-to-many `person_media` junction table
2. ✅ Media management UI (attach, view, edit, delete, manage links)
3. ✅ Person Edit dialog with "Manage Photos & Media" button
4. ✅ Media viewer/gallery for each person
5. ✅ **Media Library** - centralized view of all media
6. ✅ Global "Add Media" with multi-person selection
7. ✅ Auto-refresh after adding media
8. ✅ Include media in backup/export operations, separate zip

### Document & Video Support (✓ Complete)
- ✅ PDF support (birth certificates, documents)
- ✅ Video support (MP4, MOV)
- ✅ Office documents (Word, Excel, PowerPoint - .doc, .docx, .xls, .xlsx, .ppt, .pptx)
- ✅ Placeholder thumbnails for non-image media (color-coded: red=documents, blue=videos)
- ✅ Open in OS default application (Preview, Office, video player, etc.)
- ✅ Temporary file creation for database-stored non-images


---

## 📊 Phase 4: Reports & Analysis

### Reports ⭐ MEDIUM PRIORITY
- ✅ **Statistics Dashboard**: Comprehensive database overview
  - ✅ Total people count, living vs. deceased ratio
  - ✅ Relationship and marriage counts
  - ✅ Relationship calculator between any two people records, linked or not
  - ✅ Data quality percentages (complete vs. incomplete)
  - ✅ Media file counts
  - ✅ Top 10 most common surnames with counts
  - ✅ Timeline span and estimated generations
- ✅ **Duplicate Detection & Merge**: Find potential duplicates based on name/date similarity with intelligent merge functionality
- ✅ **Conflicts Report**: Impossible dates (married before birth, died before birth, parent too young/old, child born after parent death, etc.)
- ✅ **Timeline View**: Visual chronological display of all life events (births, deaths, marriages) with year grouping and event icons
- ✅ **Descendant Report**: All descendants of selected person with generation counts and interactive navigation
- ✅ **Ancestor Report**: All ancestors of selected person in ahnentafel numbering format with pedigree collapse detection
- ✅ **Family Group Sheet**: Traditional nuclear family report showing parents, marriage details, and all children with spouses; alternatively shows parental family with siblings for unmarried individuals
- ✅ **Missing Information Report**: List people with missing birth/death/marriage data (Data Quality Report)
- ✅ **Age & Lifespan Statistics**: Oldest living person, oldest deceased person, and average lifespan calculations in Statistics Dashboard
- ✅ **Geographic Distribution Report**: Birth and death locations grouped by place with counts, interactive navigation to people born/died in each location


### Interactive Reports
- ✅ Click any person in any in-application report to navigate to them in the main dialog
**Note:** Phase 4 is functionally complete. The following features are moved to Phase 11 (see below).

---

## Phase 5: Gramps Import Advanced Features (Future)
- ✅ We have equivalent features, but not currently Gramps import
- ⏳ Import source citations from Gramps
- ⏳ Import full event system data from Gramps
- ⏳ Import research logs from Gramps

### Advanced Media Features (Future)
- ⏳ Face tagging (link faces in photos to people)
- ⏳ Photo timeline view
- ⏳ Automatic photo organization by date/person
- ⏳ Batch photo import with metadata parsing
- ⏳ Video thumbnails from first frame

### Document & Video Support (Not Completed Parts)
- ⏳ Document scanning integration (Future)
- ⏳ OCR for searchable documents (Future)

---

## 🔍 Phase 6: Research Tools

### Research Management
- ✅ **Recent People & Bookmarks**: Track recently accessed people (up to 20) and bookmark/star important people for quick access
  - ✅ Automatic tracking of person access with timestamps
  - ✅ Bookmark toggle button in toolbar (⭐ Bookmark / ★ Bookmarked)
  - ✅ Recent People report showing last 20 accessed people with timestamps
  - ✅ Bookmarked People report for quick access to starred individuals
  - ✅ Keyboard shortcut (Cmd/Ctrl+W) for toggling bookmarks
  - ✅ Visual indicators (★) throughout the application:
    - ✅ Main people list (left panel)
    - ✅ Family View (all people displayed)
    - ✅ Pedigree View (all ancestors)
    - ✅ Individual View (table listing)
    - ✅ All reports and dialogs
- ✅ **Research To-Do List**: Per-person research task tracking
  - ✅ Create, edit, delete todo items for each person
  - ✅ Priority levels (low/medium/high) with color coding (🟢🟡🔴)
  - ✅ Mark tasks as completed/pending with timestamps
  - ✅ Additional notes field for each todo
  - ✅ "All Pending To-Dos" report grouped by priority
  - ✅ Visual indicators (📝) for people with pending tasks:
    - ✅ Main people list (left panel)
    - ✅ Family View (all people)
    - ✅ Pedigree View (all ancestors)
    - ✅ Individual View (table)
  - ✅ Access via person edit dialog ("📝 Manage Research To-Do" button)
  - ✅ One-click navigation from todo reports to person records
- ✅ **Source Citations**: Link records to sources/documents
  - ✅ Sources Library for managing all sources
  - ✅ Source types: vital records, census, church records, books, websites, other
  - ✅ Comprehensive source fields: author, publication, repository, call number/URL, notes
  - ✅ Citation manager per person - link sources with specific details
  - ✅ Citation fields: page/line/entry details, transcription, confidence level (low/medium/high), notes
  - ✅ Visual indicators (📚) for people with sources
  - ✅ "People Without Sources" report - identify documentation gaps
  - ✅ View all citations for a source
  - ✅ Access via Family View ("📚 Manage Sources" button)
  - ✅ Integration throughout all views
- ✅ **Research Log**: Track research activities and sources checked
  - ✅ Record what you searched, where, and what you found
  - ✅ Search date, repository/location, record type
  - ✅ Search goal and results tracking
  - ✅ Optional linking to specific people
  - ✅ Research Log manager (Media → Research Log)
  - ✅ Per-person research log view
  - ✅ "Recent Research Activity" report
  - ✅ Access via Family View ("🔍 Research Log" button)
  - ✅ Never duplicate your research efforts
- ✅ **Research Notes and Tools**: Project-wide research tracking
  - ✅ **Project Notes / Scratchpad** (v1.5.1): Database-backed notes for overall project
  - ✅ **People with Research Logs Report** (v1.5.1): See all people with research documentation
  - ✅ Complete research workflow (to-dos, logs, reports, project notes)
- **✅ Date Calculator**: Calculate time spans between dates (similar to PAF)
  - ✅ Start date and end date (calendar picker or manual entry)
  - ✅ Display years, months, and days elapsed
  - ✅ Useful for: calculating ages at events, time between births/deaths, validating timelines
  - ✅ Access via Tools menu
- **✅ Mapping**: Map capability to use APIs and draw a map with markers indicating people locations
- ✅ **Independent Relationship Calculator** (✓ Completed in v1.1 - calculate relationships between any two people with common ancestor detection) 

### Data Quality Tools
- ✅ **Standardization Helper** (v1.3.0): Global Search & Replace for place names
  - ✅ Find and replace across all place fields
  - ✅ Case-sensitive/insensitive search
  - ✅ Preview before applying changes
  - ✅ Batch standardization (e.g., "USA" → "United States")
- ✅ **Name Standardization** (v1.3.0): Name Case Conversion tool
  - ✅ Smart proper case conversion
  - ✅ Handles McDonald, O'Brien, van der Berg patterns
  - ✅ Roman numerals and suffixes (Jr., Sr., Esq.)
  - ✅ Preview before applying
- ✅ **Relationship Validator** (v1.2.0): Conflicts Report
  - ✅ Detects impossible dates (death before birth)
  - ✅ Identifies impossible relationships (parent born after child)
  - ✅ Review/validation system
  - ✅ Mark conflicts as reviewed to hide from reports
- ✅ **Duplicate Detection** (v1.2.0): Intelligent matching with merge capability
  - ✅ Name similarity matching
  - ✅ Birth date comparison
  - ✅ Smart scoring algorithm
  - ✅ Manual merge capability
  - ✅ Review system to mark false positives
- **✅ Date Format Validator**: Smart date parsing and validation (v1.5.3)
  - ✅ Real-time validation with visual feedback
  - ✅ Calendar picker integration
  - ✅ Support for genealogy date formats (abt, bef, aft)
  - ✅ Multiple format support (YYYY-MM-DD, DD Mon YYYY, etc.)
- **✅ Name Variations**: Track alternate spellings/names (v1.5.3)
  - ✅ Database table for alternate names
  - ✅ Management UI in person editor
  - ✅ Search integration
  - ✅ Display in person views
  - ✅ Support for nicknames, maiden names, spellings, etc.

---

## 🤝 Phase 7: Collaboration & Sharing

### Sharing Features
- **✅ Selective Branch Export**: Already implemented
- **⏳ Change Tracking**: Audit log of all edits (who, when, what)
- **⏳ GEDCOM Merge**: Import another GEDCOM and resolve conflicts
  - ⏳ Side-by-side comparison
  - ⏳ Manual conflict resolution
  - ⏳ Smart duplicate detection
- **⏳ Collaborative Editing**: Lock mechanism for multi-user scenarios
- ✅ **Family Website Generator** (v1.4.0): Complete multi-page static HTML website from database
  - ✅ Individual person pages with photos and relationships
  - ✅ Places index page (geographic organization)
  - ✅ Timeline page (chronological events)
  - ✅ Statistics dashboard with charts
  - ✅ Sources page (organized by type)
  - ✅ JavaScript search (real-time client-side)
  - ✅ Privacy controls (hide/limit living people)
  - ✅ Photo & media embedding (Base64)
  - ✅ Responsive design with modern CSS

### Privacy & Security
- ✅ **Privacy Flags** (v1.0+): Person.IsLiving field marks people as living
  - ✅ Used throughout application for privacy-sensitive operations
  - ✅ Manual toggle per person in edit dialog
  - ✅ Living Status Report to identify inconsistencies
- ✅ **Living People Protection** (v1.2+): Bulk mark people as living
  - ✅ "Mass Mark Living" tool (Reports → Actions)
  - ✅ Identify people likely living based on birth dates
  - ✅ Bulk update living status with age-based detection
- ✅ **Selective Export** (v1.4.0+): Export with comprehensive privacy controls
  - ✅ Privacy dialog for HTML/PDF exports
  - ✅ Two privacy modes: "Hide all living" or "Show limited info for living"
  - ✅ Birth dates/places withheld for living people
  - ✅ Marriage details hidden for living couples
  - ✅ Privacy notes displayed throughout exports
  - ✅ GDPR and privacy law compliance
- **⏳ Password Protection**: Optional database encryption (future)

---

## 💡 Phase 8: Modern UX Enhancements

### User Experience
- ✅ **Keyboard Shortcuts**: Quick navigation and common actions (16 shortcuts implemented)
  - ✅ Ctrl/Cmd+F: Focus search/filter
  - ✅ Ctrl/Cmd+N: New person
  - ✅ Ctrl/Cmd+E: Edit current person
  - ✅ Ctrl/Cmd+G: Go to Focus User
  - ✅ Ctrl/Cmd+1/2/3: Switch views
  - ✅ Ctrl/Cmd+T: Statistics Dashboard
  - ✅ Ctrl/Cmd+R: Data Quality Report
  - ✅ Ctrl/Cmd+D: Delete person
  - ✅ Ctrl/Cmd+O: Open database
  - ✅ Ctrl/Cmd+P / Ctrl/Cmd+,: Settings
  - ✅ Ctrl/Cmd+Q: Quit application
  - ✅ **Keyboard Shortcuts Preferences**: Full customization with conflict detection and reset to defaults
- ✅ **Recent People**: Quick access to recently viewed records (Completed in v1.2)
- ✅ **Bookmarks/Favorites**: Star important people for quick access (Completed in v1.2)
- ✅ **Advanced Search**: Multi-field search with filters (Completed in v1.3)
  - ✅ Name, date range, place, gender criteria
  - ✅ Boolean filters (living, deceased, has media, sources, todos, bookmarked)
  - ✅ Export results to CSV, JSON, XML
  - ✅ Keyboard shortcut: Cmd/Ctrl+Shift+F
- ✅ **Quick Actions / Context Menus** (Completed in v1.3): Right-click menus for common actions
  - ✅ Available on all person boxes in every view
  - ✅ Navigation shortcuts (Set Focus, View in Fan/Descendant Chart)
  - ✅ Quick actions (Bookmark, To-Dos, Sources, Research Log)
  - ✅ Copy information (Name, ID)
  - ✅ Generate reports (Ancestor, Descendant)
- ✅ **Global Search and Replace** (Completed in v1.3): Find and replace text across all records
  - ✅ Search for specific place names, dates, or text in notes
  - ✅ Replace all occurrences (e.g., "New York" → "New York, USA")
  - ✅ Search in Birth/Death Place, Address, City, State, Country, Notes, or All Place Fields
  - ✅ Case-sensitive option
  - ✅ Preview all matches before applying changes
  - ✅ Useful for standardizing place names, correcting spelling errors
- ✅ **Global Name Case Conversion** (Completed in v1.3): Fix inconsistent capitalization in names
  - ✅ Convert names to Proper Case, UPPERCASE, or lowercase
  - ✅ Handle special cases:
    - ✅ Name prefixes: McDonald, O'Brien, van der Berg
    - ✅ Roman numerals: I, II, III, IV, V (stay uppercase)
    - ✅ Suffixes: Jr., Sr., Esq., PhD, MD (proper format)
  - ✅ Apply to Given Names, Surnames, Both, or Preferred Names
  - ✅ Preview all affected records before applying
  - ✅ Useful after importing data with uppercase-only names
- ✅ **Undo/Redo** (v1.5.2): Revert recent changes (edit person, delete, add relationships)
  - ✅ Core system fully integrated into all edit operations
  - ✅ Edit Menu with Undo/Redo items
  - ✅ Keyboard shortcuts (Cmd/Ctrl+Z for Undo, Cmd/Ctrl+Shift+Z for Redo)
  - ✅ 50-operation history buffer
  - ✅ Support for person edits, deletions, and relationship changes
  - ✅ Automatic UI state updates
  - ✅ Thread-safe implementation

### Visualization
- ✅ **Enhanced Pedigree View** (Completed in v1.3):
  - ✅ Expand to 4-8 generations dynamically
  - ✅ Zoom controls (50-200%)
  - ✅ Color coding (by gender, living status, data completeness)
  - ✅ Collapsible branches
  - ✅ Context menus
- ✅ **Fan Chart** (Completed in v1.3): Circular ancestor view
  - ✅ Concentric rings for each generation (3-6 generations)
  - ✅ Gender-based color coding (pink/blue)
  - ✅ Enhanced Mode (show spouses & children)
  - ✅ Interactive navigation (click boxes)
  - ✅ Dynamic bidirectional synchronization
  - ✅ Context menus
  - ✅ Legend and help system
  - ✅ Laptop-friendly compact layout
- ✅ **Descendant Chart** (Completed in v1.3): Visual tree of descendants
  - ✅ Dynamic generations (3-8)
  - ✅ Color coding (by gender, living status, data completeness)
  - ✅ Zoom controls (50-200%)
  - ✅ Enhanced Mode (show spouses grouped with children)
  - ✅ Collapsible branches
  - ✅ Dynamic bidirectional synchronization
  - ✅ Context menus
- ✅ **Timeline View**: Horizontal timeline of person's life events (Completed in v1.4.0)
- ✅ **Map View** (v1.5.0 & v1.5.1): Geographic visualization of life events - COMPLETE
  - ✅ OpenStreetMap integration with interactive markers
  - ✅ Multi-person views (Person, All People, Descendants, Ancestors)
  - ✅ Advanced filtering (surname, date range, living status)
  - ✅ Generation-based color coding
  - ✅ Geocoding system with database caching
  - ✅ Geographic statistics and analysis reports
  - ✅ Migration distance calculations
  - ✅ 18 zoom levels with auto-centering
  - ✅ Marker statistics display
  - ⏳ Future enhancements: Timeline slider, heat maps, and historical overlays (Phase 7+)

### Data Entry Helpers
- **✅ Smart Date Entry**: Parse "May 1945", "1945", "abt 1945", etc. (v1.5.3)
  - ✅ Real-time date validation
  - ✅ Calendar picker integration
  - ✅ Support for genealogy date qualifiers (abt, bef, aft, circa)
  - ✅ Multiple format parsing (YYYY-MM-DD, DD Mon YYYY, Mon YYYY, YYYY, M/D/YYYY)
- **✅ Place Autocomplete**: Suggest standard place names as you type (v1.6)
  - ✅ Inline dropdown for place name entry
  - ✅ Search existing places in database
  - ✅ Real-time suggestions as user types
  - ✅ Apply to all place fields
- **✅ Name Authority**: Suggest standard name formats (v1.7)
  - ✅ Given name autocomplete with frequency counts
  - ✅ Surname autocomplete with frequency counts
  - ✅ Apply to all name entry fields (Person Edit, Advanced Search, Alternate Names)
  - ✅ Shows usage count for common names (e.g., "John (15)")
  - ✅ Improves data consistency and reduces typos
- **⏳ Relationship Suggestions**: AI-assisted relationship detection from notes (future)

---

## 🔧 Phase 9: Advanced Features

### Performance & Scalability (Future)
- ✅ **Indexed Search**: Full-text search across all fields (✓ Completed in v1.8)
- **⏳ Large Database Optimization**: Handle 10,000+ people efficiently
- **⏳ Lazy Loading**: Load views progressively for large datasets
- **⏳ Background Processing**: Import/export in background threads

### Integration (Future)
- **⏳ Online Search Integration**: Quick links to Ancestry, FamilySearch, etc.
- **⏳ Census Data Import**: Import from online census records
- **⏳ DNA Integration**: Link to DNA test results (23andMe, Ancestry DNA)
- **⏳ Calendar Integration**: Export birthdays/anniversaries to calendar

### Data Management
- ✅ **Database Maintenance** (✓ Completed in v1.2): 
  - ✅ Vacuum/optimize database
  - ✅ Check integrity
  - ✅ Repair broken relationships
  - ✅ Remove orphaned records
  - ✅ Remove duplicate relationships
  - ✅ Compact database and reclaim space
- **Batch Operations** (✓ Complete): 
  - ✅ Bulk mark as living (v1.2.0)
  - ✅ Bulk edit place names (v1.3.0): Global Search & Replace
  - ✅ Bulk standardize names (v1.3.0): Name Case Conversion tool
  - ✅ Batch geocoding (v1.5.0): Geocoding Tool
  - ✅ Bulk delete operations (v1.5.3): Not impelemented separately: Available via cascade delete when deleting individuals
  - ✅ Bulk privacy settings (v1.5.3): Mark as living/deceased, clear contact info, clear death dates

---

## 📱 Phase 10: Future Platforms (Deferred)

### ⏳ Mobile Apps (Ambitious - Future)
- **iOS/Android Apps**: Read-only view of database (sync via iCloud/Dropbox)
- **Web Version**: Browser-based interface for viewing only
- **Progressive Web App**: Offline-capable web version

### ⏳ Cloud Sync (Optional - Future)
- **Optional Cloud Backup**: Auto-backup to user's cloud storage
- **Multi-Device Sync**: Keep databases in sync across devices
- **Collaboration Server**: Optional self-hosted server for family collaboration


---

## 📄 Phase 11: Report Export & Templates (✓ Core Features Complete in v1.4.0)

### ✅ Report Export ⭐ PRACTICAL (Completed in v1.4.0)
- ✅ Export reports to PDF format (Family Group Sheet, Descendant, Ancestor, Timeline)
- ✅ Export data to CSV for spreadsheet analysis (People, Timeline, Surnames, Places, Sources)
- ✅ Export reports and website to HTML with embedded CSS
- ✅ Privacy options: Hide sensitive data for living people (birth date, address, phone, email)
- ✅ Complete multi-page website generation with navigation
- ✅ Places Index, Timeline Page, Statistics Dashboard, Source Citations Pages
- ✅ JavaScript real-time search (client-side)
- ✅ Photo & media embedding with base64 encoding

### ⏳ Customizable Report Templates (Optional/Low Priority - Future)
- Custom report templates with user-defined layouts
- Template editor UI
- Save/load template files
- Preview before generating

**Why Low Priority:**
- Complex to implement (template system, UI editor)
- Most users happy with default reports
- Can achieve similar results with export → edit in Word/Excel
- Estimated effort: 3-4 weeks
- **Recommendation:** Defer until user demand justifies the complexity

---

## 🎯 Priority Summary

### ✅ Completed Core Features (v1.0, 1.1, 1.2, 1.3)
1. ✅ Gramps Import - Phase 1 & 2 (Core Data + Media)
2. ✅ Media Support - Complete (blob storage, thumbnails, gallery, library)
3. ✅ Statistics Dashboard
4. ✅ Keyboard Shortcuts (23 shortcuts with full customization)
5. ✅ Additional Reports (Timeline, Duplicates, Conflicts, Ancestor, Descendant)
6. ✅ Geographic Distribution Report
7. ✅ Independent Relationship Calculator - Calculate relationships between any two people with common ancestor analysis
8. ✅ Recent People & Bookmarks - Quick access to recently viewed and bookmarked people
9. ✅ Research To-Do List - Per-person task tracking with priority levels and completion status
10. ✅ Source Citations - Link sources to people with detailed citation management
11. ✅ Research Log - Track research activities, sources checked, and search results
12. ✅ Database Maintenance Tools - Vacuum, integrity check, orphan cleanup, statistics, and analysis
13. ✅ Advanced Search - Multi-field search with filters and export to CSV/JSON/XML (v1.3)
14. ✅ Enhanced Pedigree View - 4-8 generations, zoom, color coding, collapsible branches (v1.3)
15. ✅ Fan Chart - Circular ancestor view with Enhanced Mode and dynamic sync (v1.3)
16. ✅ Quick Actions / Context Menus - Right-click menus across all views (v1.3)
17. ✅ Descendant Chart - Visual tree with Enhanced Mode and dynamic sync (v1.3)
18. ✅ Global Search and Replace - Batch text replacement across all records (v1.3)
19. ✅ Global Name Case Conversion - Fix inconsistent name capitalization with smart handling (v1.3)
20. ✅ HTML Export & Website Generation - Complete multi-page website with privacy controls (v1.4.0)
21. ✅ Places Index, Timeline, Statistics Dashboard, Source Citations Pages (v1.4.0)
22. ✅ JavaScript Search - Real-time client-side search functionality (v1.4.0)
23. ✅ Photo & Media Embedding in HTML exports with base64 encoding (v1.4.0)
24. ✅ PDF Export - Professional PDF generation for all reports (v1.4.0)
25. ✅ CSV Export - Data exports for People, Timeline, Surnames, Places, Sources (v1.4.0)

### ✅ Current Release (v1.4.0)

**✅ Phase 11 - HTML Export & Website Generation** ⭐ (✓ Completed)

See detailed release notes in ReleaseNotes.txt for complete feature list and usage instructions.

### ⏳ Previous Releases

**✅ v1.3 - Phase 8 & 9** ⭐ (✓ Completed)
- **Phase 8 - UX Enhancements** (Advanced Search, Enhanced Pedigree, Fan Chart, Context Menus, Descendant Chart)
- **Phase 9 - Professional Reports & Export Tools** (Date Calculator, Saved Searches, Chart Exports)

**Advanced Search (✓ Completed in v1.3)**
1. ✅ Multi-field search with name, date ranges, places, gender
2. ✅ Boolean filters (living, deceased, has media, sources, todos, bookmarked)
3. ✅ Export results to CSV, JSON, or XML
4. ✅ Keyboard shortcut: Cmd/Ctrl+Shift+F
5. ✅ Accessible from Reports menu and keyboard

**Enhanced Pedigree View (✓ Completed in v1.3)**
1. ✅ Dynamic generation selection (4-8 generations)
2. ✅ Zoom controls (50-200% slider)
3. ✅ Color coding modes (none, gender, living status, data completeness)
4. ✅ Collapsible branches with per-person toggle
5. ✅ Control panel with all settings at top
6. ✅ Expand All button to restore full view

**Fan Chart (✓ Completed in v1.3)**
1. ✅ Circular ancestor visualization in 360-degree layout
2. ✅ Generation selection (3-6 generations)
3. ✅ Gender-based color coding (pink/blue/gray)
4. ✅ Enhanced Mode: Show spouses and children
5. ✅ Dynamic bidirectional synchronization with Family View
6. ✅ Interactive navigation (click any box)
7. ✅ Context menus on all person boxes
8. ✅ Legend and help system
9. ✅ Compact, laptop-friendly window (700x700)

**Quick Actions / Context Menus (✓ Completed in v1.3)**
1. ✅ Right-click context menus on all person boxes (all views)
2. ✅ Navigation shortcuts (Set Focus, View in Fan/Descendant Chart)
3. ✅ Quick actions (Bookmark, To-Dos, Sources, Research Log)
4. ✅ Copy information (Name, ID)
5. ✅ Generate reports (Ancestor, Descendant)
6. ✅ Custom TappableContainer widget for dual-click support

**Descendant Chart (✓ Completed in v1.3)**
1. ✅ Visual tree showing descendants flowing downward
2. ✅ Dynamic generation selection (3-8 generations)
3. ✅ Color coding (gender, living status, data completeness)
4. ✅ Zoom controls (50-200%)
5. ✅ Enhanced Mode: Show spouses grouped with children
6. ✅ Collapsible branches
7. ✅ Dynamic bidirectional synchronization with Family View
8. ✅ Context menus on all person boxes
9. ✅ Multiple marriages clearly indicated

**Global Search and Replace (✓ Completed in v1.3)**
1. ✅ Find and replace text across all records
2. ✅ Search in Birth Place, Death Place, Address, City, State, Country, Notes, or All Place Fields
3. ✅ Case-sensitive option
4. ✅ Preview all matching records before applying changes
5. ✅ Batch replacement for standardizing place names (e.g., "USA" → "United States")
6. ✅ Useful for correcting spelling errors across many records
7. ✅ Accessible from Tools menu
8. ✅ Safe with preview-before-apply workflow

**Global Name Case Conversion (✓ Completed in v1.3)**
1. ✅ Convert names to Proper Case, UPPERCASE, or lowercase
2. ✅ Smart handling of special cases:
   - ✅ McDonald, MacArthur (Mc/Mac prefix capitalization)
   - ✅ O'Brien, O'Connor (apostrophe handling)
   - ✅ van der Berg, von Schmidt (lowercase prefixes)
   - ✅ Roman numerals: I, II, III, IV, V, VI, VII, VIII, IX, X (stay uppercase)
   - ✅ Suffixes: Jr., Sr., Esq., PhD, MD, DDS (proper format with/without periods)
3. ✅ Apply to Given Names only, Surnames only, Both, or Preferred Names
4. ✅ Preview all affected records before applying
5. ✅ Perfect for fixing imported GEDCOM files with "JOHN SMITH" or "stanley i yelnats"
6. ✅ Accessible from Tools menu
7. ✅ Safe with preview-before-apply workflow

**Professional Chart Exports (✓ Completed in v1.3)**
1. ✅ High-quality PDF and PNG exports for all chart types
2. ✅ Pedigree Chart Export:
   - ✅ Columnar, generational layout matching in-app view
   - ✅ Correct Ahnentafel numbering with nil placeholders
   - ✅ Up to 4 generations with proper positioning
   - ✅ Wider root person box (name truncation fix)
   - ✅ Gender-based color coding
3. ✅ Fan Chart Export:
   - ✅ Pedigree-style columnar format for print clarity
   - ✅ Correct ancestor positioning
   - ✅ All generations properly aligned
4. ✅ Descendant Chart Export:
   - ✅ Hierarchical tree structure with proper parent-child lines
   - ✅ Recursive tree building from database
   - ✅ Dynamic box sizing (wider root, default children)
   - ✅ Correct positioning and scaling
5. ✅ Export Features:
   - ✅ Chart overview text (clean, ASCII-only)
   - ✅ High-resolution output
   - ✅ Accessible from all chart views
   - ✅ File save dialogs with correct extensions

**Family Group Sheet (✓ Completed in v1.3)**
1. ✅ Traditional genealogy report format
2. ✅ Married Couple View:
   - ✅ Husband and wife sections with full details
   - ✅ Marriage information (date, place from relationship record)
   - ✅ All children chronologically ordered with:
     * Birth and death dates/places
     * Spouse names
     * Grandchildren listed with birth dates
   - ✅ Multiple marriage support (chronologically ordered)
3. ✅ Parental Family View:
   - ✅ Parents section
   - ✅ Person details
   - ✅ All siblings chronologically ordered
4. ✅ Access Points:
   - ✅ Reports menu → Family Group Sheet
   - ✅ System tray menu → Reports → Family Group Sheet
   - ✅ Right-click any person → Family Group Sheet
   - ✅ Reports toolbar button
   - ✅ Cmd/Ctrl+R → Reports menu
5. ✅ Interactive navigation to all family members

**Enhanced Quick Access Toolbar (✓ Completed in v1.3)**
1. ✅ Six comprehensive popup menu buttons:
   - ✅ **📁 File** (11 items): New, Open, Recent Files (nested submenu), Backup, Maintenance, Restore, Import/Export
   - ✅ **🖼️ Media** (4-5 items): Add, Library, Research Log, Sources, View Media (count)
   - ✅ **🔧 Tools** (3 items): Date Calculator, Search & Replace, Name Case Conversion
   - ✅ **📊 Reports** (6 categories, 24 items): Hierarchical nested submenus
   - ✅ **⚙️ Settings** (6 items): Settings, Shortcuts, Theme options
   - ✅ **❓ Help** (4 items): About, Updates, Help, Demo Database
2. ✅ Hierarchical Reports Structure:
   - ✅ Utilities (3): Advanced Search, Relationship Calculator, Statistics
   - ✅ Lists (5): To-Dos, Bookmarked People, Recent People/Research
   - ✅ Data Quality (7): Sources, Conflicts, Duplicates, Living Status, etc.
   - ✅ Individual Reports (4): Ancestor, Descendant, Family Group Sheet, Timeline
   - ✅ Chart Views (3): Descendant Chart, Fan Chart, Geographic Distribution
   - ✅ Actions (2): Mass Mark Living, Reviewed Items
3. ✅ 100% feature parity with full menus
4. ✅ Cmd/Ctrl+R keyboard shortcut for Reports menu
5. ✅ All menus alphabetically organized within categories

**Enhanced Context Menus (✓ Completed in v1.3)**
1. ✅ View Media in pedigree view right-click menu
2. ✅ Shows media count when available
3. ✅ Quick access to person's photos/documents from any chart view

### ✅ Previous Release (v1.2)

**Phase 6 & Phase 9 - Research Tools & Database Maintenance** ⭐

**Phase 6: Research Tools**
1. ✅ Recent People / Bookmarks (Completed in v1.2)
2. ✅ Research To-Do List (Completed in v1.2)
3. ✅ Source Citations (Completed in v1.2)
4. ✅ Research Log (Completed in v1.2)
5. ✅ Preferred Name Field (Completed in v1.2.1)

**Phase 9: Database Maintenance (✓ Completed in v1.2)** ⭐
1. ✅ Database Maintenance Tools:
   - ✅ Vacuum/optimize database for better performance
   - ✅ Check integrity with SQLite PRAGMA integrity_check
   - ✅ Remove orphaned records (media, relationships without people)
   - ✅ Remove duplicate relationships
   - ✅ Compact database and reclaim space
2. ✅ Database Analysis:
   - ✅ Show database file size and comprehensive statistics
   - ✅ Identify orphaned media and unused sources
   - ✅ Report on relationship consistency
   - ✅ Optimize query performance with ANALYZE
3. ✅ UI Features:
   - ✅ Separate resizable window for maintenance operations
   - ✅ Three-tab interface (Statistics, Maintenance, Analysis)
   - ✅ Asynchronous operations (non-blocking UI)
   - ✅ Detailed reporting with duration tracking
   - ✅ Keyboard shortcut: Cmd/Ctrl+L

### ✅ Completed in v1.3: Phase 8 & 9 - UX Enhancements & Professional Reports

**1. ✅ Advanced Search** ⭐ HIGH VALUE (Completed)
- ✅ Multi-field search (name, dates, places, gender)
- ✅ Date range searches (birth/death year ranges)
- ✅ Place searches (birth/death places with contains matching)
- ✅ Filter by: living status, has media, has sources, has todos, bookmarked
- ✅ Export search results to CSV, JSON, or XML
- ✅ Keyboard shortcut: Cmd/Ctrl+Shift+F
- ✅ Accessible from Reports menu
- ✅ Save search criteria for reuse

**2. ✅ Enhanced Pedigree View** ⭐ HIGH VALUE (Completed)
- ✅ Expand to 4-8 generations dynamically (user selectable)
- ✅ Zoom controls (50% to 200% with slider)
- ✅ Color coding options:
  - ✅ By gender (blue/pink/gray)
  - ✅ By living status (green/gray)
  - ✅ By data completeness (green/yellow/pink for 80%/50%/<50%)
- ✅ Collapsible branches (per-person collapse/expand with ↕ button)
- ✅ Control panel at top (generations, colors, zoom, expand all)
- ✅ All visual indicators preserved (★📷📚📝🔍)
- ✅ Export to PDF/PNG (Completed in v1.3)

**3. ✅ Fan Chart** 🌟 WOW FEATURE (Completed)
- ✅ Circular ancestor visualization in 360-degree layout
- ✅ Interactive boxes (click to navigate to any ancestor)
- ✅ Gender-based color coding (pink/blue/gray)
- ✅ Configurable generation depth (3-6 generations)
- ✅ Enhanced Mode: Show spouses and children alongside ancestors
- ✅ Dynamic bidirectional synchronization with Family View
- ✅ Compact, laptop-friendly layout (700x700 default window)
- ✅ Context menus on all person boxes
- ✅ Beautiful visual representation with adaptive sizing
- ✅ Legend and help system
- ✅ Export to PDF/PNG (Completed in v1.3)

**4. ✅ Quick Actions / Context Menus** ⭐ HIGH VALUE (Completed)
- ✅ Right-click context menus on all person boxes (all views)
- ✅ Navigation shortcuts (Set Focus, View in Fan/Descendant Chart)
- ✅ Quick actions (Bookmark, To-Dos, Sources, Research Log)
- ✅ Copy information (Name, ID to clipboard)
- ✅ Generate reports (Ancestor, Descendant for any person)
- ✅ Universal implementation across Family, Pedigree, Fan, Descendant views
- ✅ Custom TappableContainer widget for dual-click support

**5. ✅ Descendant Chart** 🌟 WOW FEATURE (Completed)
- ✅ Visual tree showing children and future generations
- ✅ Dynamic generations (3-8 selectable)
- ✅ Color coding (gender, living status, data completeness)
- ✅ Zoom controls (50-200%)
- ✅ Enhanced Mode: Show spouses grouped with their children
- ✅ Collapsible branches (focus on specific descendant lines)
- ✅ Dynamic bidirectional synchronization with Family View
- ✅ Context menus on all person boxes
- ✅ Multiple marriages clearly indicated
- ✅ Export to PDF/PNG (Completed in v1.3)

### 🎯 Next Priority: Phase 10 & 11 - Additional Export Formats (v1.4)

**Phase 11: HTML Reports & Privacy Controls** ⭐ HIGH VALUE (✓ Completed in v1.4.0)
1. ✅ HTML report generation for individual reports (Family Group Sheet, Descendants, Ancestors)
2. ✅ Privacy settings for living people (hide completely or show limited information)
3. ✅ Generate complete shareable multi-page HTML family websites
4. ✅ Export individual/family reports to HTML with embedded CSS and photos
5. ✅ Places Index Page with geographic organization
6. ✅ Timeline Page with chronological events and living section
7. ✅ Statistics Dashboard with interactive charts and hyperlinks
8. ✅ Source Citations Page organized by type
9. ✅ JavaScript real-time search (client-side, no server required)
10. ✅ Photo & media embedding with base64 encoding
11. ✅ Enhanced navigation (breadcrumbs, feature cards, folder creation)
12. ✅ PDF export for all reports (Family Group Sheet, Descendant, Ancestor, Timeline)
13. ✅ CSV export for data analysis (People, Timeline, Surnames, Places, Sources)

**Phase 10: Additional Platform Support** (Deferred)
1. ⏳ Mobile companion apps (read-only database view)
2. ⏳ Cloud sync capabilities (optional)
3. ⏳ Cross-platform compatibility enhancements

### ✅ Phase 12: Map Visualization (v1.5.0 - Released January 30, 2026)

**Phase 12: Map View** 🗺️ WOW FEATURE ⭐ HIGH PRIORITY - COMPLETE

**See MAP_IMPLEMENTATION_PLAN.md for detailed technical specifications.**

**Completed Features (v1.5.0):**
1. ✅ **Core Map Integration** (Phase 1) - COMPLETE
   - ✅ OpenStreetMap integration with Fyne UI
   - ✅ Interactive map window with zoom controls (18 levels)
   - ✅ Standalone map view accessible from Person Details
   - ✅ Control panel for marker filters and display options
   - ✅ Internet connectivity checking with user-friendly messages

2. ✅ **Geocoding System** (Phase 2) - COMPLETE
   - ✅ Place name to coordinates conversion using Nominatim API
   - ✅ SQLite database caching to avoid repeated API calls
   - ✅ Batch geocoding tool with progress dialog (Tools → Geocoding Tool)
   - ✅ Rate limiting to respect Nominatim usage policy
   - ✅ Automatic geocoding for existing place names

3. ✅ **Life Event Markers** (Phase 3) - COMPLETE
   - ✅ Birth markers (🟢 green circle)
   - ✅ Death markers (⚫ black circle)
   - ✅ Marriage markers (💒 pink heart)
   - ✅ Click markers to view detailed event information
   - ✅ Toggle marker visibility by event type
   - ✅ Re-center map on specific event types

4. ✅ **Person-Specific Map View** (Phase 4) - COMPLETE
   - ✅ Individual life event visualization
   - ✅ Auto-center on birth location
   - ✅ "Show on Map" button in Person Details dialog
   - ✅ Status bar showing coordinates and zoom level
   - ✅ Clean, optimized rendering with thread-safe UI updates

**Actual Effort:** ~8-10 hours (including debugging and polish)

**Extended Features (v1.5.1 - Released January 31, 2026):**
5. ✅ **Multi-Person & Family Views** (Phase 5) - COMPLETE
   - ✅ All people mode with comprehensive filtering system
   - ✅ Descendants/Ancestors mode showing family expansion/origins
   - ✅ Surname filter for branch-specific visualization
   - ✅ Date range filter (from/to years) for temporal analysis
   - ✅ Living/Deceased status filter
   - ✅ Color-coding by generation (blue→green→red gradient)
   - ✅ Real-time marker statistics display (Births/Deaths/Marriages)
   - ✅ Enhanced UI with two-row control panel
   - ✅ Apply Filters button for explicit control
   - ✅ Smart filtering of unknown/placeholder place names
   - ⏳ Heat map overlay (deferred to Phase 7)

6. ✅ **Geographic Reports & Analysis** (Phase 6) - COMPLETE
   - ✅ Geographic statistics in Statistics Dashboard
   - ✅ Migration Distance Report with Haversine calculations
   - ✅ Geographic Hotspots Report (top countries/states/cities)
   - ✅ Cross-Border Families Report (multi-country events)
   - ✅ Unmapped Places Report (geocoding gaps)
   - ✅ New "Geographic Reports" submenu in Reports menu
   - ✅ Distance displayed in kilometers and miles
   - ✅ Birth/death place statistics with percentages

**Actual Effort (Phases 5 & 6):** ~10-12 hours

---

### 🎯 Released: v1.6 - Quick Wins (Released February 3, 2026)

**Focus:** High-impact, foundational improvements that benefit all users

**Backup Reminders** ⭐ HIGH PRIORITY (✓ Complete)
- ✅ Track last backup date in database settings
- ✅ Check on app startup (configurable days threshold, default: 7)
- ✅ Reminder dialog with "Backup Now" option
- ✅ Settings toggle to enable/disable reminders
- ✅ Update tracking when user creates backup via app
- ✅ Prevents data loss through gentle reminders
- ✅ Optimized Settings UI layout (backups at top, clear focus user above search)
- **User Value:** VERY HIGH - Critical data safety feature
- **Actual Effort:** 2-3 hours (including UI refinements)

**Phase 8: Place Autocomplete** ⭐ HIGH PRIORITY (✓ Complete)
- ✅ Inline dropdown for place name entry (clean, dialog-friendly UI)
- ✅ Search existing places in database (birth, death, marriage places)
- ✅ Real-time suggestions as user types (2+ characters)
- ✅ Apply to all place fields (person birth/death, marriage/union)
- ✅ Comprehensive database query (UNION across all place fields)
- ✅ Improves data quality and consistency automatically
- ⏳ Fuzzy matching deferred (works well without it)
- ⏳ GeoNames API integration deferred (database suggestions sufficient)
- **User Value:** VERY HIGH - Daily quality-of-life improvement
- **Actual Effort:** 3-4 hours (including popup→inline redesign)

**Phase 9: Performance & Scalability** ⭐ HIGH PRIORITY (✓ Core Complete)
- ✅ **Database indexing on commonly queried fields** (automatic on migration)
  - ✅ Names (given, surname, preferred)
  - ✅ Dates (birth_date, death_date, birth_year, death_year)
  - ✅ Places (birth_place, death_place, marriage_place)
  - ✅ Filters (is_living, gender, uid)
  - ✅ Relationships (type for spouse/partner queries)
- ✅ **Startup optimization**: Combined backup reminder queries (3 queries → 1)
- ✅ **Query planner optimization**: PRAGMA optimize for smart index usage
- ⏳ Lazy loading for large lists (not needed yet, deferred)
  - People list (load in chunks of 100-200)
  - Media library (paginated loading)
  - Report results (virtual scrolling)
- ⏳ Profile current bottlenecks with benchmarking (deferred)
- ⏳ Memory optimization for large databases (deferred - not needed yet)
- ⏳ Background processing for non-critical tasks (deferred)
- **User Value:** HIGH - Future-proofs for growth, strengthens "fast vs Gramps" competitive advantage
- **Actual Effort:** 2 hours (core indexing complete, advanced features deferred)
- **Performance Results:** ✅ Verified fast on 1,400 people, ready for 10,000+

**Total Actual Effort:** 7-9 hours (significantly under original 30-45 hour estimate)
**Release Date:** February 3, 2026

---

### 🎯 Released: v1.7 - Name Authority (Released February 5, 2026)

**Focus:** Complete Phase 8 Data Entry Helpers with intelligent name suggestions

**Phase 8: Name Authority** ⭐ HIGH PRIORITY (✓ Complete)
- ✅ **Given Name Autocomplete**: Suggests existing given names as you type
  - ✅ Searches all given names and preferred names in database
  - ✅ Shows frequency count for common names (e.g., "John (15)")
  - ✅ Real-time suggestions with 2+ character trigger
  - ✅ Smart ranking: most common names appear first
- ✅ **Surname Autocomplete**: Suggests existing surnames as you type
  - ✅ Searches all surnames in database
  - ✅ Shows frequency count for family names
  - ✅ Helps maintain consistent spelling across family branches
- ✅ **Universal Integration**: Applied to all name entry locations
  - ✅ Person Edit Dialog (main edit screen)
  - ✅ Advanced Search (name criteria fields)
  - ✅ Alternate Names Manager (both add and edit dialogs)
- ✅ **Data Quality Benefits**:
  - ✅ Reduces typos and spelling variations
  - ✅ Encourages consistent name formatting
  - ✅ Makes it easy to use established family names
  - ✅ Improves searchability and reports
- **User Value:** VERY HIGH - Daily quality-of-life improvement, completes Phase 8
- **Actual Effort:** 4-5 hours (including all integration points)

**Total Actual Effort:** 4-5 hours
**Release Date:** February 5, 2026

---

### 🎯 Recent Release: v1.7 - Name Authority & Map Enhancements (February 5, 2026)

In addition to Name Authority, v1.7 includes valuable map improvements:

**Phase 7: Map Enhancements** ⭐ HIGH VALUE (✓ Complete)
- ✅ **Marriage Location Markers**: Show all marriage places for a person
  - ✅ 💒 Pink heart markers for each marriage
  - ✅ Click marker to see marriage details with spouse names
  - ✅ Age at marriage displayed in popup
  - ✅ Integrated into person-specific map view
- ✅ **Current Address for Living People**: Show where living people are now
  - ✅ 🟢 Green marker for current address
  - ✅ Uses City/Address/State/Country fields from contact info
  - ✅ Labeled as "Current Address" with "Present" as date
  - ✅ Combines address fields for accurate geocoding
- ✅ **Migration Paths**: Draw lines connecting life events chronologically
  - ✅ Blue lines for deceased: birth → death
  - ✅ Green lines for living: birth → current address
  - ✅ Visual representation of person's life journey
  - ✅ Toggle on/off with "🔀 Migration Paths" checkbox
  - ✅ Paths drawn underneath markers for clarity
- ✅ **Age Display**: Show person's age at each life event
  - ✅ Calculated age shown in marker popups
  - ✅ "Age: 85 years" displayed for death events
  - ✅ Age at marriage also calculated and shown
  - ✅ Handles incomplete dates gracefully
- ✅ **Quick Access**: Already available via context menus
  - ✅ Right-click any person → "View on Map..."
  - ✅ Opens map centered on person's birth location
  - ✅ Available in Family View, Pedigree View, Fan Chart, Descendant Chart
- **User Value:** VERY HIGH - Complete life journey visualization for all people
- **Actual Effort:** 4-5 hours

**Phase 8: Historical Place Names** ⭐ HIGH VALUE (✓ Complete)
- ✅ **Database & Models**: Alternate place name mappings
  - ✅ `alternate_places` table with historical → current mappings
  - ✅ CRUD operations in store
  - ✅ Index on historical names for fast lookups
- ✅ **Management UI**: Tools → Historical Place Names
  - ✅ Add/Edit/Delete place name mappings
  - ✅ Examples: "Salisbury, Rhodesia" → "Harare, Zimbabwe"
  - ✅ Optional year changed and notes fields
  - ✅ Clear instructions and examples in UI
- ✅ **Geocoding Fallback**: Automatic retry with modern name
  - ✅ When historical place not found, tries modern equivalent
  - ✅ Saves successful geocode with note about fallback
  - ✅ Dramatically improves geocoding success rate
- ✅ **Display Helper**: Format place names with context
  - ✅ Helper function `formatPlaceWithHistoricalContext()`
  - ✅ Can be integrated throughout UI as needed
  - ✅ Shows as: "Salisbury, Rhodesia (now Harare, Zimbabwe)"
- **User Value:** VERY HIGH - Maintains historical accuracy while enabling modern functionality
- **Actual Effort:** 5-6 hours

**Phase 9: Automatic Background Geocoding** ⭐ HIGH VALUE (✓ Complete)
- ✅ **Background Service**: Automatic geocoding without user intervention
  - ✅ Starts 10 seconds after app launch
  - ✅ Finds all un-geocoded places in database
  - ✅ Respects 1-second rate limit (Nominatim API requirements)
  - ✅ Runs quietly in background thread
  - ✅ Rechecks every 30 minutes for new places
- ✅ **Smart Place Detection**: Finds all place types
  - ✅ Birth and death places from persons table
  - ✅ Marriage places from relationships table
  - ✅ Current addresses for living people (combines address fields)
  - ✅ Skips already-geocoded and placeholder places
- ✅ **Logging**: Console output for monitoring
  - ✅ Shows start/stop messages
  - ✅ Logs progress: "Successfully geocoded 'Durban, South Africa' (5/10)"
  - ✅ Reports success/failure counts
- ✅ **Lifecycle Management**: Proper start/stop on database changes
  - ✅ Stops old geocoder when switching databases
  - ✅ Starts new geocoder for new database
  - ✅ Thread-safe with mutex protection
- **User Value:** VERY HIGH - "Set it and forget it" - places are automatically geocoded
- **Actual Effort:** 2-3 hours

**Total Actual Effort (v1.7):** 14-18 hours (Name Authority + Map Enhancements + Alternate Names + Historical Places + Auto Geocoding)
**Release Date:** February 5, 2026

---

### 🎯 Released: v1.8 - Quick Wins & Full-Text Search (Released February 5, 2026)

**Focus:** High-impact UX improvements and powerful search capabilities

**UX Improvements** ⭐ HIGH VALUE (✓ Complete)
- ✅ **Map Export to PNG/PDF**: Export maps with all markers and paths
  - ✅ 1920x1080 resolution for presentations and reports
  - ✅ PNG format for web/documents, PDF for printing
  - ✅ Parallel tile downloading with progress dialog
  - ✅ Respects API rate limits (max 10 concurrent requests)
  - ✅ Captures all visible markers, paths, and legend
- ✅ **Map Panning**: Navigate maps with arrow buttons
  - ✅ Four directional arrow buttons (up/down/left/right)
  - ✅ Smooth panning without conflicting with marker clicks
  - ✅ Simpler than drag-based panning (no event conflicts)
- ✅ **Family View Scroll**: Fixed large family display issue
  - ✅ Proper vertical scroll container for families with many children
  - ✅ Window remains resizable and movable
  - ✅ Handles families with 10+ siblings smoothly
- ✅ **Focused Person Context Menu**: Right-click on focused person
  - ✅ Consistent behavior across all views
  - ✅ Same context menu as other family members
- ✅ **Global Search Default**: Changed to "All Place Fields"
  - ✅ More useful default than "Birth Place" only
  - ✅ Finds matches across all location fields
- ✅ **Window Tracking**: Prevent duplicate dialog windows
  - ✅ Global Search & Replace tracks window state
  - ✅ Historical Place Names tracks window state
  - ✅ Brings existing window to front instead of opening duplicates
- ✅ **Geocoding Bug Fix**: Retry with historical names on failures
  - ✅ Checks alternate places even for cached "failed" geocodes
  - ✅ Re-attempts geocoding when user adds historical mappings
- **User Value:** HIGH - Quality-of-life improvements for daily use
- **Actual Effort:** 6-8 hours

**Full-Text Search** ⭐ HIGH VALUE (✓ Complete)
- ✅ **SQLite FTS5 Virtual Table**: Blazing-fast indexed search
  - ✅ Porter stemming for smart matching ("photographer" finds "photo")
  - ✅ Unicode support for international names and places
  - ✅ Automatic index population from existing data
  - ✅ Schema upgrade detection (auto-recreates on enhancement)
- ✅ **Comprehensive Search**: Searches all text fields
  - ✅ Names (given, surname, preferred)
  - ✅ Places (birth, death, address, city, state, country)
  - ✅ **Marriage places** (all marriages for a person)
  - ✅ **Spouse names** (find people by spouse's name)
  - ✅ Notes (full-text search across all person notes)
  - ✅ Contact info (email, phone)
  - ✅ UID (find by unique identifier)
- ✅ **Powerful Query Syntax**: User-friendly but powerful
  - ✅ **AND logic** (default): "Durban South Africa" finds all three words
  - ✅ **OR logic**: "London OR Paris" finds either city
  - ✅ **NOT logic**: "Durban -Natal" excludes results with Natal
  - ✅ **Exact phrases**: "military service" (with quotes)
  - ✅ **Prefix matching**: "photo*" finds photo, photographer, photography
  - ✅ **Complex boolean**: "Durban AND (photographer OR artist)"
  - ✅ **Case insensitive**: All searches ignore case automatically
  - ✅ **In-app help**: Expandable "Advanced Query Syntax" guide
- ✅ **Smart Results Display**: Context-aware result presentation
  - ✅ Shows matched field (e.g., "Notes", "Birth Place", "Marriage Place", "Spouse Name")
  - ✅ Snippet with highlighted match context
  - ✅ Relevance ranking (most relevant results first)
  - ✅ Click to navigate directly to person
- ✅ **UI Integration**: Easy access throughout app
  - ✅ Tools → Full-Text Search menu item
  - ✅ Keyboard shortcut: Cmd/Ctrl+Shift+T
  - ✅ Tools popup menu
  - ✅ Single-instance window (brings to front if already open)
  - ✅ Helpful examples in placeholder text
  - ✅ Quick tips always visible
  - ✅ Expandable advanced syntax reference
- ✅ **Real-time Index Updates**: Always current
  - ✅ Auto-updates FTS index on person create/update/delete
  - ✅ Auto-updates FTS index on relationship create/update/delete
  - ✅ Updates both subject and object persons when relationships change
  - ✅ Rebuild function for manual refresh if needed
- **User Value:** VERY HIGH - Find anything instantly with powerful yet simple query syntax
- **Actual Effort:** 7-8 hours (including relationship indexing and query syntax documentation)

**Total Actual Effort (v1.8):** 12-15 hours (UX improvements + Full-Text Search with relationships)
**Release Date:** February 5, 2026

---

### 📋 Future Releases - Medium Priority (v1.9+)

**Phase 7: Advanced Map Features** (Future Polish & Enhancements)
- ⏳ Timeline slider: Show where person was at different ages
- ⏳ Animated paths: Trace journey over time with animation
- ⏳ Heat map overlay: Density visualization of family events
- ⏳ Measure tool: Calculate distances between any two locations
- ✅ Export map as image (PNG, PDF) with legend (✓ Completed in v1.8)
- ⏳ Historical map overlays: Show borders as they were in different eras
- ⏳ Street view integration: Link to Google Street View for precise locations
- **Estimated Effort:** 15-20 hours (multiple releases)

**Phase 8: Data Entry Helpers** (Mostly Complete)
- ✅ Smart Date Entry: Parse "May 1945", "1945", "abt 1945", etc. (✓ Completed in v1.5.3)
- ✅ Place Autocomplete: Suggest standard place names as you type (✓ Completed in v1.6)
- ✅ Name Authority: Suggest standard name formats (✓ Completed in v1.7)
- ⏳ Relationship Suggestions: AI-assisted relationship detection from notes
- **Actual Effort:** 4-5 hours for Name Authority

**Phase 9: Performance & Scalability**
- ✅ Indexed Search: Full-text search across all fields (✓ Completed in v1.8)
- Large Database Optimization: Handle 10,000+ people efficiently
- Lazy Loading: Load views progressively for large datasets
- Background Processing: Import/export in background threads
- **Estimated Effort:** 20-30 hours (reduced with FTS5 completion)

---

### 📋 Future Releases - Low Priority (v1.7+)

**Phase 9: Batch Operations** (Mostly Complete)
- ✅ Bulk mark as living (v1.2.0)
- ✅ Bulk edit place names (v1.3.0): Global Search & Replace
- ✅ Bulk standardize names (v1.3.0): Name Case Conversion
- ✅ Batch geocoding (v1.5.0): Geocoding Tool
- ✅ Bulk delete operations - not implemented, see notes related to cascade delete individual
- ✅ More comprehensive bulk privacy settings
- **Actual Effort:** 8-10 hours (most features complete)

**Phase 8: Advanced UX** ✅ (Completed in v1.5.2)
- ✅ Undo/Redo functionality (complex but valuable)
- ✅ Command history tracking (50-operation buffer)
- ✅ Edit reversal for person changes, deletions, relationship modifications
- ✅ Keyboard shortcuts (Cmd/Ctrl+Z, Cmd/Ctrl+Shift+Z)
- ✅ Edit menu integration
- **Actual Effort:** ~20 hours (core system existed, needed integration)

**Phase 7: Collaboration & Sharing**
1. GEDCOM Merge with conflict resolution
2. Change Tracking (audit log of all edits)
3. Advanced Privacy Features (privacy flags, automatic protection)
4. Collaborative Editing (lock mechanism)
- **Estimated Effort:** 30-40 hours

**Phase 9: Integration**
- Online Search Integration: Quick links to Ancestry, FamilySearch, etc.
- Census Data Import: Import from online census records
- DNA Integration: Link to DNA test results (23andMe, Ancestry DNA)
- Calendar Integration: Export birthdays/anniversaries to calendar
- **Estimated Effort:** 25-35 hours

---

### 📋 Deferred Features (v2.0+)

**Phase 5: Gramps Import Advanced Features** (When Needed)
- Import source citations from Gramps
- Import full event system data from Gramps
- Import research logs from Gramps
- Note: Pending investigation of Gramps data availability

**Phase 10: Mobile & Web Platforms** (Ambitious)
- iOS/Android companion apps (read-only database view)
- Web version with browser-based interface
- Progressive Web App (offline-capable)
- **Estimated Effort:** 100+ hours

**Phase 10: Cloud Sync** (Optional)
- Optional cloud backup (user's cloud storage)
- Multi-device sync capabilities
- Collaboration server (self-hosted for families)
- **Estimated Effort:** 80+ hours

**Phase 8: Advanced Map Features** (v1.6+)
- Historical map overlays (borders as they were in different eras)
- 3D terrain view
- Street view integration
- Weather data at events
- Travel time estimates between locations
- **Estimated Effort:** 20-30 hours

**Advanced AI-Assisted Features** (Future)
- Automatic relationship detection from notes
- Smart data entry predictions
- Duplicate detection using ML
- Record matching with online databases
- **Estimated Effort:** 50+ hours

---

## 🌍 Future Considerations (Not Scheduled)

### Internationalization (i18n) & Language Packs

**Strategic Approach:** Build the best genealogy app first, then make it accessible to more languages.

**When to Revisit:**
- User demand emerges (3+ requests from non-English users)
- After v1.8-1.9 when major features are mature
- Identified market opportunity in specific international genealogy communities
- Natural lull between major feature releases

**Phased Implementation Plan:**
1. **Phase 1: Infrastructure** (10-15 hours)
   - Set up translation framework (Fyne i18n support)
   - Extract menu items only (~50-100 strings)
   - Add 1-2 languages (Spanish, French)
   - Language selector in Settings
   - Test user interest/feedback

2. **Phase 2: Full UI Coverage** (40-60 hours)
   - Extract all UI strings (buttons, labels, dialogs)
   - Expand to 3-5 common languages
   - Handle date/number formatting
   - UI layout adjustments for text expansion

3. **Phase 3: Complete Localization** (60-80 hours)
   - All reports and exports
   - Help text and tooltips
   - Error messages and notifications
   - Community contribution system

**Priority Languages for Genealogy:**
- Spanish (Latin America, Spain)
- French (France, Canada, Belgium)
- German (Germany, Austria, Switzerland)
- Portuguese (Brazil, Portugal)
- Italian, Polish, Dutch, Chinese (based on demand)

**Development Guidelines (Current):**
- Keep string externalization in mind during development
- Avoid deeply embedding UI text in business logic
- Keep messages/labels organized near top of files
- Makes future extraction easier without slowing current development

**Total Estimated Effort:** 110-155 hours (full implementation)
**Recommended Start:** After v1.8+ when feature set stabilizes

---

## 📝 Notes

**Design Principles:**
- **Simplicity First**: Don't replicate every feature of Gramps/GenoPro/others if it adds complexity
- **Modern UX**: Fast, intuitive, beautiful interface
- **Single-File Portability**: Database (+ embedded media) should be one file for easy backup
- **Standards Compliance**: GEDCOM compatibility for interoperability
- **Cross-Platform**: Must work identically on Mac, Windows, Linux

**Performance Targets:**
- App launch: < 2 seconds
- Database load (1000 people): < 1 second
- View switching: < 200ms
- Search results: < 500ms

**Success Metrics:**
- Faster than Gramps for all operations
- More intuitive than PAF
- Feature-complete enough to replace both for 90% of users

---

*Last Updated: March 3, 2026 - Roadmap aligned with v1.8 status and corrected completed/pending items*

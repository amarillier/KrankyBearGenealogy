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
- Most applications (Family Tree Maker, Ancestry.com, MyHeritage, etc.) export to GEDCOM
- Our robust GEDCOM import handles these already
- Only implemented GenoPro and Gramps because they have unique benefits:
  - GenoPro: Proprietary XML format users may not know how to export
  - Gramps: SQLite database preserves more structure and notes

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

## Phase 5: Gramps Import Advanced Features (Future)##
- Source citations
- Full event system
- Research logs

### Advanced Media Features (Future)
- Face tagging (link faces in photos to people)
- Photo timeline view
- Automatic photo organization by date/person
- Batch photo import with metadata parsing
- Video thumbnails from first frame

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
    - Main people list (left panel)
    - Family View (all people displayed)
    - Pedigree View (all ancestors)
    - Individual View (table listing)
    - All reports and dialogs
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
- **Research Notes and Tools**: Separate from person notes, track research progress
- **Date Calculator**: Calculate time spans between dates (similar to PAF)
  - Start date and end date (calendar picker or manual entry)
  - Display years, months, and days elapsed
  - Useful for: calculating ages at events, time between births/deaths, validating timelines
  - Access via Tools menu
  - Estimated effort: 1-2 hours
- **Mapping**: Map capability to use APIs and draw a map with markers indicating people locations?
- ✅ **Independent Relationship Calculator** (✓ Completed in v1.1 - calculate relationships between any two people with common ancestor detection) 

### Data Quality Tools
- **Standardization Helper**: Suggest standard place names
- **Date Format Validator**: Ensure consistent date formats
- **Relationship Validator**: Check for impossible relationships
- **Name Variations**: Track alternate spellings/names

---

## 🤝 Phase 7: Collaboration & Sharing

### Sharing Features
- **Selective Branch Export**: Already implemented ✅
- **Change Tracking**: Audit log of all edits (who, when, what)
- **GEDCOM Merge**: Import another GEDCOM and resolve conflicts
  - Side-by-side comparison
  - Manual conflict resolution
  - Smart duplicate detection
- **Collaborative Editing**: Lock mechanism for multi-user scenarios
- **Family Website Generator**: Create static HTML site from database

### Privacy & Security
- **Privacy Flags**: Mark people as private (exclude from exports/reports)
- **Living People Protection**: Auto-mark as private if born < 100 years ago
- **Password Protection**: Optional database encryption
- **Selective Export**: Export with privacy filters applied

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
- **Undo/Redo**: Revert recent changes (edit person, delete, add relationships)
  - Core system implemented, needs integration hooks in edit dialogs
  - Deferred for future release

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
- **Timeline View**: Horizontal timeline of person's life events (Already implemented ✅)
- **Map View**: Geographic visualization of life events

### Data Entry Helpers
- **Smart Date Entry**: Parse "May 1945", "1945", "abt 1945", etc.
- **Place Autocomplete**: Suggest standard place names as you type
- **Name Authority**: Suggest standard name formats
- **Relationship Suggestions**: AI-assisted relationship detection from notes

---

## 🔧 Phase 9: Advanced Features

### Performance & Scalability
- **Indexed Search**: Full-text search across all fields
- **Large Database Optimization**: Handle 10,000+ people efficiently
- **Lazy Loading**: Load views progressively for large datasets
- **Background Processing**: Import/export in background threads

### Integration
- **Online Search Integration**: Quick links to Ancestry, FamilySearch, etc.
- **Census Data Import**: Import from online census records
- **DNA Integration**: Link to DNA test results (23andMe, Ancestry DNA)
- **Calendar Integration**: Export birthdays/anniversaries to calendar

### Data Management
- **Database Maintenance**: 
  - Vacuum/optimize database
  - Check integrity
  - Repair broken relationships
  - Remove orphaned records
- **Batch Operations**: 
  - Bulk edit (change place names, standardize dates)
  - Bulk delete
  - Bulk privacy settings
- **Data Migration Tools**: Convert between different schemas/versions

---

## 📱 Phase 10: Future Platforms

### Mobile Apps (Ambitious)
- **iOS/Android Apps**: Read-only view of database (sync via iCloud/Dropbox)
- **Web Version**: Browser-based interface for viewing only
- **Progressive Web App**: Offline-capable web version

### Cloud Sync (Optional)
- **Optional Cloud Backup**: Auto-backup to user's cloud storage
- **Multi-Device Sync**: Keep databases in sync across devices
- **Collaboration Server**: Optional self-hosted server for family collaboration


---

## 📄 Phase 11: Report Export & Templates (Lower Priority)

### Report Export ⭐ PRACTICAL
- Export reports to PDF format (Statistics, Conflicts, Data Quality, etc.)
- Export reports to CSV for spreadsheet analysis
- Export reports to HTML for web sharing
- Privacy options: Hide sensitive data for living people (birth date, address, phone, email)
- Batch export (export all reports at once)

**Implementation Notes:**
- PDF: Use a Go PDF library (e.g., gofpdf, go-pdf)
- CSV: Built-in Go csv package
- HTML: Use Go's html/template
- Estimated effort: 1-2 weeks

### Customizable Report Templates (Optional/Low Priority)
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

**Phase 11 - HTML Export & Website Generation** ⭐ (✓ Completed)

See detailed release notes in ReleaseNotes.txt for complete feature list and usage instructions.

### ⏳ Previous Releases

**v1.3 - Phase 8 & 9** ⭐ (✓ Completed)
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

**Phase 10: Additional Platform Support**
1. ⏳ Mobile companion apps (read-only database view)
2. ⏳ Cloud sync capabilities (optional)
3. ⏳ Cross-platform compatibility enhancements

### Future Releases (v1.4+)

**Phase 6: Additional Research Tools** (Future)
- **✅ Date Calculator**: Time span calculator for genealogy research
  - ✅ Calculate years/months/days between any two dates
  - ✅ Calendar picker or manual date entry
  - ✅ Useful for age calculations and timeline validation
  - ✅ Estimated effort: 1-2 hours

**Phase 6: Mapping** 🗺️ WOW FEATURE (Future)
- Map visualization of life events
- Geographic timeline
- Migration paths
- API integration (Google Maps or OpenStreetMap)
- Geocoding for place names
- Note: Deferred until ready for polished implementation

**Phase 8: Advanced UX** (Continued)
- Undo/Redo functionality (complex but valuable)

**Phase 7: Collaboration**
1. GEDCOM Merge with conflict resolution
2. Change Tracking (audit log)
3. Privacy Features (privacy flags, living people protection)

**Phase 11: Report Export** (✓ Completed in v1.4.0 - see Phase 11 above)
1. ✅ Export reports to PDF (Family Group Sheet, Descendant, Ancestor, Timeline)
2. ✅ Export data to CSV (People, Timeline, Surnames, Places, Sources)
3. ✅ Export reports and website to HTML with embedded CSS

**Phase 9: Performance**
1. Performance Optimizations for large databases (10,000+ people)

**Phase 5: Gramps Import Advanced Features** (When Needed)
- Import source citations from Gramps
- Import full event system data from Gramps
- Import research logs from Gramps
- Note: Pending investigation of Gramps data availability

**Phase 10:Future Platfoms and other features**
**Long Term (Ambitious)**
1. Mobile/Web Versions
2. Cloud Sync
3. AI-Assisted Features
4. DNA Integration

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

*Last Updated: January 29, 2026 - v1.4.0 Released*

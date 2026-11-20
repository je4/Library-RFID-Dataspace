### Mermaid Graph Visualization Specification

This document defines the schema, styling conventions, and concrete examples for rendering interactive Mermaid graph visualizations of physical RFID/NFC library items and their corresponding OpenAlex scholarly knowledge graph relations.

---

### 1. Visualization Architecture & UI Integration

When physical RFID tags are read via WebSocket, the dataspace constructs a graph model representing the physical Alma holdings and their enriched OpenAlex metadata. The backend/frontend translates this graph into a standard Mermaid `flowchart TD` definition.

```mermaid
flowchart TD
    WS["NFC WebSocket Events<br/>(UID, Barcode)"] --> Aggregator["Graph Aggregator Service<br/>• Barcode Lookups (fhnw_alma-full)<br/>• Entity Matcher (openalex-*)<br/>• Relation Builder"]
    Aggregator --> Generator["Mermaid DSL Generator<br/>• Node & Edge Builder<br/>• Subgraph Partitions<br/>• Semantic Styling Classes"]
    Generator --> WebUI["Web UI Container<br/>• DaisyUI Card / Modal<br/>• mermaid.js SVG Render<br/>• Click Events & Drawer Details"]

    classDef proc fill:#2563eb,stroke:#1d4ed8,stroke-width:2px,color:#ffffff;
    classDef io fill:#10b981,stroke:#047857,stroke-width:2px,color:#ffffff;
    class WS,WebUI io;
    class Aggregator,Generator proc;
```

---

### 2. Node Schema & Visual Styling

To ensure clarity and accessibility, nodes are visually categorized using semantic classes and shapes:

| Entity Type | Shape | CSS Class | Color Scheme / Meaning |
| :--- | :--- | :--- | :--- |
| **Scanned RFID Tag** | Stadium `([Label])` | `nodeTag` | Bright Emerald / Active physical scan |
| **Alma Catalog Record** | Rounded Rectangle `[Label]` | `nodeAlma` | Royal Blue / Physical library holding |
| **InK Digital Archive / Asset** | Rectangle `[Label]` | `nodeInk` | Rose / Mediathek InK digital asset / collection |
| **OpenAlex Work** | Rounded Rectangle `[Label]` | `nodeWork` | Deep Indigo / Scholarly publication |
| **Author / Scholar** | Hexagon `{{Label}}` | `nodeAuthor` | Amber / Researcher & creator |
| **Research Topic** | Cylinder `[(Label)]` | `nodeTopic` | Purple / Specific research topic |
| **Academic Discipline** | Subroutine `[[Label]]` | `nodeTaxonomy`| Cyan / Subfield, Field, Domain |
| **Institution / Library** | Rectangle `[Label]` | `nodeInst` | Slate / University or library branch |
| **Publisher / Source** | Parallelogram `[/Label/]` | `nodeSource` | Teal / Journal or publishing house |

#### Visual Node Legend & Style Definitions

```mermaid
flowchart LR
    Tag(["nodeTag: RFID Tag"]):::nodeTag
    Alma["nodeAlma: Alma Record"]:::nodeAlma
    Ink["nodeInk: InK Media / Archive"]:::nodeInk
    Work["nodeWork: OpenAlex Work"]:::nodeWork
    Author{{"nodeAuthor: Author"}}:::nodeAuthor
    Topic[("nodeTopic: Topic")]:::nodeTopic
    Taxonomy[["nodeTaxonomy: Discipline"]]:::nodeTaxonomy
    Inst["nodeInst: Institution"]:::nodeInst
    Source[/"nodeSource: Publisher / Source"/]:::nodeSource

    classDef nodeTag fill:#10b981,stroke:#047857,stroke-width:3px,color:#ffffff,font-weight:bold;
    classDef nodeAlma fill:#2563eb,stroke:#1d4ed8,stroke-width:2px,color:#ffffff;
    classDef nodeInk fill:#db2777,stroke:#9d174d,stroke-width:2px,color:#ffffff;
    classDef nodeWork fill:#4f46e5,stroke:#4338ca,stroke-width:2px,color:#ffffff;
    classDef nodeAuthor fill:#d97706,stroke:#b45309,stroke-width:2px,color:#ffffff;
    classDef nodeTopic fill:#7c3aed,stroke:#6d28d9,stroke-width:2px,color:#ffffff;
    classDef nodeTaxonomy fill:#0891b2,stroke:#0e7490,stroke-width:2px,color:#ffffff;
    classDef nodeInst fill:#475569,stroke:#334155,stroke-width:2px,color:#ffffff;
    classDef nodeSource fill:#0d9488,stroke:#0f766e,stroke-width:2px,color:#ffffff;
```

```text
%% Reusable Mermaid classDef snippet for Dataspace Graphs
classDef nodeTag fill:#10b981,stroke:#047857,stroke-width:3px,color:#ffffff,font-weight:bold;
classDef nodeAlma fill:#2563eb,stroke:#1d4ed8,stroke-width:2px,color:#ffffff;
classDef nodeInk fill:#db2777,stroke:#9d174d,stroke-width:2px,color:#ffffff;
classDef nodeWork fill:#4f46e5,stroke:#4338ca,stroke-width:2px,color:#ffffff;
classDef nodeAuthor fill:#d97706,stroke:#b45309,stroke-width:2px,color:#ffffff;
classDef nodeTopic fill:#7c3aed,stroke:#6d28d9,stroke-width:2px,color:#ffffff;
classDef nodeTaxonomy fill:#0891b2,stroke:#0e7490,stroke-width:2px,color:#ffffff;
classDef nodeInst fill:#475569,stroke:#334155,stroke-width:2px,color:#ffffff;
classDef nodeSource fill:#0d9488,stroke:#0f766e,stroke-width:2px,color:#ffffff;
```

---

### 3. Concrete Example 1: Single Scanned Physical Book

The following example visualizes a single scanned physical book (*Heinrich Hertz: Erinnerungen, Briefe, Tagebücher*), linking the physical item to its Alma bibliographic record, author, holding library, related Mediathek InK digital repository assets, and the corresponding OpenAlex work, topic hierarchy, and academic institution.

```mermaid
flowchart TD
    %% Subgraph: Physical Library Space
    subgraph PhysicalSpace ["Physical Library Space (Alma)"]
        Tag1(["NFC Tag: 0001847294"]):::nodeTag
        AlmaBook1["Alma Book: Heinrich Hertz (2010)<br/>ISBN: 978-3-8171-3420-1"]:::nodeAlma
        Library1["FHNW Campus Library Muttenz<br/>ISIL: CH-001884-6"]:::nodeInst
        Shelf1["Shelf: 530.092 HER / EG-104"]:::nodeAlma
    end

    %% Subgraph: InK Digital Repository
    subgraph InkSpace ["Mediathek InK & Archive (HGK FHNW)"]
        InkItem1["InK: Historical Audio Archive<br/>Typ: soundRecording<br/>Sig: zotero2-4278174.QNUBU85W"]:::nodeInk
        InkColl1["Collection: Archiv HGK Audio / Video"]:::nodeInk
        InkPlatform["Mediathek InK HGK<br/>mediathek.hgk.fhnw.ch/ink/de"]:::nodeInk
    end

    %% Subgraph: Scholarly Graph
    subgraph ScholarlyGraph ["Scholarly Knowledge Graph (OpenAlex)"]
        Work1["OpenAlex Work: Heinrich Hertz Papers<br/>DOI: 10.1007/978-3-642-12345-6"]:::nodeWork
        Author1{{"Author: Albrecht Fölsing"}}:::nodeAuthor
        AuthorHertz{{"Historical Figure: Heinrich Hertz"}}:::nodeAuthor
        Inst1["University of Hamburg<br/>ROR: 00g30e956"]:::nodeInst
        Publisher1[/"Publisher: Springer Nature"/]:::nodeSource
    end

    %% Subgraph: Disciplinary Hierarchy
    subgraph Taxonomy ["Disciplinary Hierarchy (OpenAlex)"]
        Topic1[("Topic: History of Electromagnetism & Physics")]:::nodeTopic
        Subfield1[["Subfield: Physics and Astronomy"]]:::nodeTaxonomy
        Field1[["Field: Physical Sciences"]]:::nodeTaxonomy
        Domain1[["Domain: Physical Sciences"]]:::nodeTaxonomy
    end

    %% Physical Space Edges
    Tag1 -->|"scanned on reader"| AlmaBook1
    AlmaBook1 -->|"held at"| Library1
    AlmaBook1 -->|"located on"| Shelf1

    %% InK Repository Edges
    AlmaBook1 -->|"digital item / media"| InkItem1
    InkItem1 -->|"part of collection"| InkColl1
    InkItem1 -.->|"repository portal"| InkPlatform
    Author1 -->|"author in InK"| InkItem1

    %% Alma to OpenAlex Bridge Edges
    AlmaBook1 -.->|"matched via ISBN / Title"| Work1
    AlmaBook1 -->|"authored by"| Author1
    AlmaBook1 -->|"subject"| AuthorHertz

    %% OpenAlex Internal Edges
    Work1 -->|"authored by"| Author1
    Work1 -->|"published by"| Publisher1
    Work1 -->|"primary topic"| Topic1
    Author1 -->|"affiliated with"| Inst1

    %% Taxonomy Traversal Edges
    Topic1 -->|"subfield of"| Subfield1
    Subfield1 -->|"field of"| Field1
    Field1 -->|"domain"| Domain1

    %% Styling classes
    classDef nodeTag fill:#10b981,stroke:#047857,stroke-width:3px,color:#ffffff,font-weight:bold;
    classDef nodeAlma fill:#2563eb,stroke:#1d4ed8,stroke-width:2px,color:#ffffff;
    classDef nodeInk fill:#db2777,stroke:#9d174d,stroke-width:2px,color:#ffffff;
    classDef nodeWork fill:#4f46e5,stroke:#4338ca,stroke-width:2px,color:#ffffff;
    classDef nodeAuthor fill:#d97706,stroke:#b45309,stroke-width:2px,color:#ffffff;
    classDef nodeTopic fill:#7c3aed,stroke:#6d28d9,stroke-width:2px,color:#ffffff;
    classDef nodeTaxonomy fill:#0891b2,stroke:#0e7490,stroke-width:2px,color:#ffffff;
    classDef nodeInst fill:#475569,stroke:#334155,stroke-width:2px,color:#ffffff;
    classDef nodeSource fill:#0d9488,stroke:#0f766e,stroke-width:2px,color:#ffffff;
```

---

### 4. Concrete Example 2: Multi-Item Co-Occurrence Intelligence

When two or more physical books are scanned simultaneously on the RFID reader desk, the graph highlights shared intersections (e.g., shared authors, cross-citations, common research topics, and joint institutional affiliations).

```mermaid
flowchart TD
    %% Physical Tags Scanned
    subgraph ScannedItems ["Scanned Physical Items on Desk"]
        TagA(["NFC Tag A: 0005128912"]):::nodeTag
        TagB(["NFC Tag B: 0009841123"]):::nodeTag
        BookA["Book A: Quantum Computing Fundamentals<br/>(Alma MMS: 9910023412)"]:::nodeAlma
        BookB["Book B: Advanced Algorithms in Cryptography<br/>(Alma MMS: 9910087654)"]:::nodeAlma
    end

    %% Shared Scholarly Entities
    subgraph Intersections ["Shared Knowledge Graph & Overlap (OpenAlex)"]
        SharedAuthor{{"Shared Contributor: Dr. Elena Fischer"}}:::nodeAuthor
        SharedTopic[("Shared Topic: Quantum Cryptographic Protocols")]:::nodeTopic
        SharedInst["ETH Zurich<br/>ROR: 05a28rw58"]:::nodeInst
        
        WorkA["OpenAlex Work A<br/>W2187391283"]:::nodeWork
        WorkB["OpenAlex Work B<br/>W2904819230"]:::nodeWork
    end

    %% Disciplinary Taxonomy
    subgraph TaxonomyTree ["Academic Discipline Tree"]
        SubfieldCS[["Subfield: Computer Science & Cryptography"]]:::nodeTaxonomy
        FieldCS[["Field: Computer Science"]]:::nodeTaxonomy
        DomainPhysical[["Domain: Physical Sciences"]]:::nodeTaxonomy
    end

    %% Physical Scans to Books
    TagA -->|"detected"| BookA
    TagB -->|"detected"| BookB

    %% Link to OpenAlex Works
    BookA -.->|"matches"| WorkA
    BookB -.->|"matches"| WorkB

    %% Shared Intersections
    BookA -->|"authored by"| SharedAuthor
    BookB -->|"co-authored by"| SharedAuthor

    WorkA -->|"primary topic"| SharedTopic
    WorkB -->|"primary topic"| SharedTopic

    WorkA ==>|"CITES WORK"| WorkB

    SharedAuthor -->|"affiliated institution"| SharedInst

    %% Taxonomy links
    SharedTopic -->|"subfield of"| SubfieldCS
    SubfieldCS -->|"field of"| FieldCS
    FieldCS -->|"domain"| DomainPhysical

    %% Styling
    classDef nodeTag fill:#10b981,stroke:#047857,stroke-width:3px,color:#ffffff,font-weight:bold;
    classDef nodeAlma fill:#2563eb,stroke:#1d4ed8,stroke-width:2px,color:#ffffff;
    classDef nodeWork fill:#4f46e5,stroke:#4338ca,stroke-width:2px,color:#ffffff;
    classDef nodeAuthor fill:#d97706,stroke:#b45309,stroke-width:2px,color:#ffffff;
    classDef nodeTopic fill:#7c3aed,stroke:#6d28d9,stroke-width:2px,color:#ffffff;
    classDef nodeTaxonomy fill:#0891b2,stroke:#0e7490,stroke-width:2px,color:#ffffff;
    classDef nodeInst fill:#475569,stroke:#334155,stroke-width:2px,color:#ffffff;
    classDef nodeSource fill:#0d9488,stroke:#0f766e,stroke-width:2px,color:#ffffff;
```

---

### 5. Deep Linking & Interactive Navigation

The Mermaid flowchart integrates actionable deep links (`click <NodeID> href "<URL>" "<Tooltip>" _blank`) directly into the knowledge graph nodes, opening targeted catalog, authority, repository, and discovery pages in new browser tabs.

| Target Platform / Entity | URL Pattern & Example | Node Target & Usage |
| :--- | :--- | :--- |
| **GND Authority (lobid.org)** | `https://lobid.org/gnd/{gndId}`<br/>*Example*: `https://lobid.org/gnd/118772295` | `nodeAuthor`, `nodeTopic` with extracted GND identifier (`(DE-588)...`) |
| **Swisscovery Library Catalog** | `https://swisscovery.ch/discovery/search?query=any,contains,{mmsIdOrSearch}&tab=41SLSP_NETWORK&search_scope=DN_and_CI&vid=41SLSP_NETWORK:VU1_UNION`<br/>*Example*: `https://swisscovery.ch/discovery/search?query=any,contains,991170430529605501&tab=41SLSP_NETWORK&search_scope=DN_and_CI&vid=41SLSP_NETWORK:VU1_UNION` | `nodeAlma` (Physical Alma record / MMS ID search) |
| **Mediathek HGK InK Detail** | `https://mediathek.hgk.fhnw.ch/ink/detail/{signature}/de`<br/>*Example*: `https://mediathek.hgk.fhnw.ch/ink/detail/zotero2-2068924.UHBBY3JP/de` | `nodeInk` (Individual media asset or student thesis) |
| **Mediathek HGK InK Portal** | `https://mediathek.hgk.fhnw.ch/ink/de` | `nodeInk` (`InkPortal`, `InkColl_*` collections) |
| **OpenAlex Works** | `https://openalex.org/works/{workId}` or `?search={isbn/title}`<br/>*Example*: `https://openalex.org/works/W642892681` | `nodeWork` (Scholarly publication node) |
| **OpenAlex Authors** | `https://openalex.org/authors/{authorId}` or `?search={name}`<br/>*Example*: `https://openalex.org/authors/A5090064378` | `nodeAuthor` (Researcher profile without direct GND) |
| **OpenAlex Institutions** | `https://openalex.org/institutions/{institutionId}` or `?search={name}`<br/>*Example*: `https://openalex.org/institutions/I118019719` | `nodeInst` (Universities, libraries, corporate contributors) |
| **OpenAlex Publishers** | `https://openalex.org/publishers/{publisherId}` or `?search={name}`<br/>*Example*: `https://openalex.org/publishers/P4310320990` | `nodeSource` (Publishing houses and parent organizations) |
| **OpenAlex Sources** | `https://openalex.org/sources/{sourceId}` or `?search={name}`<br/>*Example*: `https://openalex.org/sources/S7407052672` | `nodeSource` (Journals, conferences, repositories) |
| **OpenAlex Types** | `https://openalex.org/types/{type}`<br/>*Example*: `https://openalex.org/types/article` | Work publication types (`article`, `book`, `thesis`) |
| **OpenAlex Institution Types** | `https://openalex.org/institution-types/{type}`<br/>*Example*: `https://openalex.org/institution-types/education` | Institutional categories (`education`, `healthcare`, `facility`) |
| **OpenAlex Source Types** | `https://openalex.org/source-types/{type}`<br/>*Example*: `https://openalex.org/source-types/repository` | Source venue categories (`repository`, `journal`, `conference`) |
| **OpenAlex Indexes** | `https://openalex.org/indexes/{index}`<br/>*Example*: `https://openalex.org/indexes/crossref` | Bibliographic indexing registries (`crossref`, `doaj`) |
| **OpenAlex Source Lists** | `https://openalex.org/source-lists/{list}`<br/>*Example*: `https://openalex.org/source-lists/cwts-core` | Curated source lists (`cwts-core`, `doaj`) |
| **OpenAlex Topics** | `https://openalex.org/topics/{topicId}` or `?search={label}` | `nodeTopic` (Faceted research topics) |
| **OpenAlex Subfields & Fields** | `https://openalex.org/subfields`, `https://openalex.org/fields`, `https://openalex.org/domains` | `nodeTaxonomy` (Taxonomy hierarchy nodes) |

---

### 6. Frontend Rendering Guidelines

1. **Client-Side Mermaid Initialization**:
   * Include Mermaid library in the web bundle (`mermaid.min.js`).
   * Initialize with dark/light theme awareness matching DaisyUI themes and `securityLevel: 'loose'` to enable hyperlink click triggers.
2. **Dynamic Generation & Interaction**:
   * When WebSocket tag updates are processed, serialize the entity graph into the Mermaid DSL string with `click <NodeID> href "<URL>" "<Tooltip>" _blank` directives.
   * Render SVG via `mermaid.render(id, dslString)` inside `#dataspace-mermaid-output`.
   * Zoom & Pan transformations are preserved seamlessly across data reloads, while drag-distance detection prevents accidental link triggers during viewport panning.

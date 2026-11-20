### Entity Relationships and Linking Strategy

This document specifies how bibliographic records retrieved from physical RFID/NFC tag scans via `fhnw_alma-full` are matched, interconnected, and bridged into the OpenAlex scholarly knowledge graph (`openalex-*`).

---

### 1. Ingestion Flow: Physical RFID Tag to Knowledge Graph

```mermaid
flowchart TD
    NFC["RFID / NFC Tag on Reader"] -->|"NFC WebSocket Event (UID, ItemID)"| Barcode["Item Barcode"]
    Barcode -->|"Elasticsearch search.identifier.keyword"| Alma["fhnw_alma-full Record"]
    
    Alma -->|"Internal Linking"| AlmaRel["Alma Relations<br/>• Same Author<br/>• Same Subject / GND<br/>• Shelf Neighbors"]
    Alma -->|"Digital Archive Bridge"| InkBridge["Mediathek InK Matcher<br/>• CallNumber / Barcode Match<br/>• Creator / Author Match<br/>• Title & Collection Match<br/>• ACL Filter: global/guest"]
    Alma -->|"Entity Resolution Bridge"| Bridge["OpenAlex Matcher<br/>• Exact DOI / ISBN Match<br/>• Title & Author Fuzzy Search<br/>• Author Name / ORCID Match<br/>• GND Subject Topic Mapping"]
    
    InkBridge -->|"Archive Expansion"| InkGraph["Mediathek InK Assets<br/>• Student Theses & Projects<br/>• Video & Audio Recordings<br/>• Institute & Archive Collections<br/>• mediathek.hgk.fhnw.ch/ink/de"]
    Bridge -->|"Graph Expansion"| OAGraph["OpenAlex Scholarly Graph<br/>• Works & Citation Chains<br/>• Authors & Affiliated Institutions<br/>• Topics, Subfields, Fields & Domains<br/>• Publishing Sources & Funders"]

    classDef accent fill:#2563eb,stroke:#1d4ed8,stroke-width:2px,color:#ffffff;
    classDef inkAccent fill:#db2777,stroke:#9d174d,stroke-width:2px,color:#ffffff;
    classDef highlight fill:#10b981,stroke:#047857,stroke-width:2px,color:#ffffff;
    classDef oaGraph fill:#7c3aed,stroke:#6d28d9,stroke-width:2px,color:#ffffff;
    class NFC,Barcode highlight;
    class Alma,AlmaRel,Bridge accent;
    class InkBridge,InkGraph inkAccent;
    class OAGraph oaGraph;
```

1. **Tag Discovery**: When an RFID tag is placed on the reader, the desk service emits a WebSocket event containing the Item ID (barcode) and UID.
2. **Catalog Resolution**: The backend queries `fhnw_alma-full` using `search.identifier.keyword` to fetch the primary MARC/MODS bibliographic record.
3. **Graph Expansion**: The system executes targeted Elasticsearch queries to discover both internal Alma relations and corresponding OpenAlex entities.

---

### 2. Physical Catalog Linking (Alma to Alma)

When one or more books are scanned, the physical catalog offers multiple dimensions of relation:

#### 2.1 Co-Authorship & Author Collections
* **Query Match**: `mapping.name.personal.*.namePart` or authority ID (`mapping.name.personal.*.identifier`).
* **Semantic Relation**: `(Book A) -- [written by] --> (Author) <-- [written by] -- (Book B)`
* **Utility**: Reveals other physical holdings by the same creator in the library network.

#### 2.2 Thematic / Subject Similarity
* **Query Match**: Exact match on GND subject identifiers (`mapping.subject.topic.*.identifier`) or fuzzy text match on subject labels (`mapping.subject.topic.*.label`, `mapping.subject.local.all`).
* **Semantic Relation**: `(Book A) -- [shares topic] --> (GND Subject) <-- [shares topic] -- (Book B)`
* **Utility**: Groups physical books that address the same disciplinary topic or artistic concept.

#### 2.3 Shelf & Location Proximity (Physical Browsing)
* **Query Match**: Common prefix on `mapping.location.holding.callNumber` and identical `mapping.location.holding.library`.
* **Semantic Relation**: `(Book A) -- [shelf neighbor at] --> (Holding Location) <-- [shelf neighbor at] -- (Book B)`
* **Utility**: Enables virtual shelf browsing of physically adjacent items.

---

### 3. Bridging Alma Records to OpenAlex Entities

Connecting physical library catalog items to OpenAlex creates an enriched scholarly graph. The bridge employs a tiered matching pipeline:

```mermaid
flowchart TD
    Start["Alma Bibliographic Record"] --> Tier1["Tier 1: Deterministic Identifiers<br/>• 024.a / recordIdentifier ➔ openalex-works.doi<br/>• identifier.isbn.enriched ➔ openalex-works.ids.isbn"]
    
    Tier1 -->|"Found Match"| ResolveWork["Direct OpenAlex Work Resolution"]
    Tier1 -->|"No Direct Identifier"| Tier2["Tier 2: Bibliographic Fuzzy Search<br/>• titleInfo.main.title ➔ openalex-works.title<br/>• name.personal.namePart ➔ openalex-works.authorships.author"]
    
    Tier2 --> ResolveWork
    
    Start --> Tier3["Tier 3: Semantic Topic & Concept Mapping<br/>• subject.topic.label ➔ openalex-topics.display_name & keywords<br/>• subject.topic.label ➔ openalex-concepts.display_name"]
    
    Start --> Tier4["Tier 4: Author & Institutional Affiliation Linking<br/>• name.personal.namePart ➔ openalex-authors.display_name<br/>• Library ISIL (FHNW) ➔ openalex-institutions (ROR: 02cs02y79)"]

    ResolveWork --> GraphIntegration["Integrated Scholarly Knowledge Graph"]
    Tier3 --> GraphIntegration
    Tier4 --> GraphIntegration

    classDef t1 fill:#2563eb,stroke:#1d4ed8,stroke-width:2px,color:#ffffff;
    classDef t2 fill:#0284c7,stroke:#0369a1,stroke-width:2px,color:#ffffff;
    classDef t3 fill:#7c3aed,stroke:#6d28d9,stroke-width:2px,color:#ffffff;
    classDef t4 fill:#d97706,stroke:#b45309,stroke-width:2px,color:#ffffff;
    classDef out fill:#10b981,stroke:#047857,stroke-width:2px,color:#ffffff;
    class Tier1 t1;
    class Tier2 t2;
    class Tier3 t3;
    class Tier4 t4;
    class ResolveWork,GraphIntegration out;
```

#### 3.1 Work Matching (Tier 1 & 2)
* **Target Index**: `openalex-works`
* **Query Structure**:
  * *Step A (Exact DOI)*: `{"term": {"doi.keyword": "<normalized_doi>"}}`
  * *Step B (ISBN)*: `{"term": {"ids.isbn.keyword": "<sanitized_isbn13>"}}`
  * *Step C (Title + Author)*:
    ```json
    {
      "query": {
        "bool": {
          "must": [
            { "match_phrase": { "title": "<alma_title>" } }
          ],
          "should": [
            { "match": { "authorships.author.display_name": "<author_name>" } }
          ]
        }
      }
    }
    ```

#### 3.2 Author Profile Resolution
* **Target Index**: `openalex-authors`
* **Query Structure**: Match author surname and given name against `display_name` and `display_name_alternatives`. If an ORCID is present in the MARC authority field, match on `orcid.keyword`.
* **Discovered Attributes**: Author h-index, total citations, historical research institutions, top publication topics.

#### 3.3 Topical and Conceptual Resolution
* **Target Indices**: `openalex-topics` and `openalex-concepts`
* **Query Structure**: Match subject strings (`mapping.subject.topic.*.label`, `mapping.subject.local.all`) against `openalex-topics.display_name` and `keywords`, or `openalex-concepts.display_name`.
* **Discovered Attributes**: Broad academic field, subfield, high-level domain, and sibling research topics.

#### 3.4 Institutional and Source Resolution
* **Target Indices**: `openalex-institutions`, `openalex-sources`, `openalex-publishers`
* **Links**:
  * Connecting Alma holding institutions (e.g., FHNW libraries) with OpenAlex Institution profiles (`https://openalex.org/I11756543`).
  * Connecting publishers (`mapping.originInfo.event.publication.publisher`) with `openalex-publishers` or `openalex-sources`.

---

### 4. OpenAlex Internal Graph Traversal

Once an OpenAlex work, author, or topic is anchored, the following second-degree relations are traversed:

| Source Entity | Relationship Edge | Target Entity | Edge Meaning |
| :--- | :--- | :--- | :--- |
| `openalex-works` | `authorships.author.id` | `openalex-authors` | "authored by" |
| `openalex-works` | `authorships.institutions.id` | `openalex-institutions` | "affiliated institution" |
| `openalex-works` | `primary_topic.id` | `openalex-topics` | "primary research topic" |
| `openalex-works` | `concepts.id` | `openalex-concepts` | "treats concept" |
| `openalex-works` | `locations.source.id` | `openalex-sources` | "published in journal/source" |
| `openalex-works` | `referenced_works[]` | `openalex-works` | "cites work" |
| `openalex-works` | `grants.funder` | `openalex-funders` | "funded by" |
| `openalex-topics` | `subfield.id` | `openalex-subfields` | "belongs to subfield" |
| `openalex-subfields` | `field.id` | `openalex-fields` | "belongs to field" |
| `openalex-fields` | `domain.id` | `openalex-domains` | "belongs to domain" |
| `openalex-topics` | `siblings.id` | `openalex-topics` | "related sibling topic" |
| `openalex-authors` | `last_known_institutions.id`| `openalex-institutions` | "current affiliation" |

---

### 5. Multi-Item Co-Occurrence Intelligence

When multiple physical books are detected simultaneously on the RFID reader (e.g., during checkout or desk interaction), the dataspace analyzes shared graph nodes to surface hidden connections:

```mermaid
flowchart TD
    subgraph PhysicalDesk ["Co-Present Items on Reader"]
        Book1["Scanned Book 1<br/>Barcode: 00012345"]:::nodeTag
        Book2["Scanned Book 2<br/>Barcode: 00067890"]:::nodeTag
    end

    subgraph DiscoveredIntersections ["Discovered Shared Scholarly Graph"]
        SharedAuthor{{"Shared Contributor<br/>Prof. M. Smith"}}:::nodeAuthor
        SharedTopic[("Shared Research Topic<br/>Renewable Energy Storage")]:::nodeTopic
        ParentField[["Parent Academic Field<br/>Energy & Engineering"]]:::nodeTaxonomy
        Work1["Scholarly Work 1"]:::nodeWork
        Work2["Scholarly Work 2"]:::nodeWork
    end

    Book1 -->|"written by"| SharedAuthor
    Book2 -->|"written by"| SharedAuthor

    Book1 -.->|"resolves to"| Work1
    Book2 -.->|"resolves to"| Work2

    Work1 -->|"primary topic"| SharedTopic
    Work2 -->|"primary topic"| SharedTopic
    SharedTopic -->|"subfield of"| ParentField

    Work1 ==>|"CITES WORK"| Work2

    classDef nodeTag fill:#10b981,stroke:#047857,stroke-width:2px,color:#ffffff,font-weight:bold;
    classDef nodeWork fill:#4f46e5,stroke:#4338ca,stroke-width:2px,color:#ffffff;
    classDef nodeAuthor fill:#d97706,stroke:#b45309,stroke-width:2px,color:#ffffff;
    classDef nodeTopic fill:#7c3aed,stroke:#6d28d9,stroke-width:2px,color:#ffffff;
    classDef nodeTaxonomy fill:#0891b2,stroke:#0e7490,stroke-width:2px,color:#ffffff;
```

1. **Shared Authors**: Detects if books were authored, edited, or illustrated by the same person.
2. **Shared Classification & Topics**: Identifies common OpenAlex topics, subfields, or GND subject terms.
3. **Citation & Influence Paths**: Finds citation links between the works or their immediate references in OpenAlex.
4. **Institutional Overlap**: Highlights common research centers or universities associated with the authors.

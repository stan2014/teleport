import { Volume, createFsFromVolume } from 'memfs';
import { TopicContentsFragment } from './gen-topic-pages.js';

describe('generate a menu page', () => {
  const testFilesTwoSections = {
    '/docs.mdx': `---
title: "Documentation Home"
description: "Guides to setting up the product."
---

Guides to setting up the product.

{/*TOPICS*/}
`,
    '/docs/database-access.mdx': `---
title: "Database Access"
description: "Guides related to Database Access."
---

Guides related to Database Access.

{/*TOPICS*/}
`,
    '/docs/database-access/page1.mdx': `---
title: "Database Access Page 1"
description: "Protecting DB 1 with Teleport"
---`,
    '/docs/database-access/page2.mdx': `---
title: "Database Access Page 2"
description: "Protecting DB 2 with Teleport"
---`,
    '/docs/application-access.mdx': `---
title: "Application Access"
description: "Guides related to Application Access"
---

Guides related to Application Access.

{/*TOPICS*/}
`,
    '/docs/application-access/page1.mdx': `---
title: "Application Access Page 1"
description: "Protecting App 1 with Teleport"
---`,
    '/docs/application-access/page2.mdx': `---
title: "Application Access Page 2"
description: "Protecting App 2 with Teleport"
---`,
  };

  test('lists the contents of a directory', () => {
    const expected = `---
title: "Database Access"
description: "Guides related to Database Access."
---

Guides related to Database Access.

{/*TOPICS*/}

- [Database Access Page 1](database-access/page1.mdx): Protecting DB 1 with Teleport
- [Database Access Page 2](database-access/page2.mdx): Protecting DB 2 with Teleport
`;

    const vol = Volume.fromJSON(testFilesTwoSections);
    const fs = createFsFromVolume(vol);
    const frag = new TopicContentsFragment(fs, '/docs/database-access');
    const actual = frag.makeTopicPage();
    expect(actual).toBe(expected);
  });

  test('treats links to directories as regular links (single)', () => {
    const expected = `---
title: Documentation Home
description: Guides for setting up the product.
---

Guides for setting up the product.

{/*TOPICS*/}

- [Application Access](docs/application-access.mdx): Guides related to Application Access
`;

    const vol = Volume.fromJSON({
      '/docs.mdx': `---
title: Documentation Home
description: Guides for setting up the product.
---

Guides for setting up the product.

{/*TOPICS*/}
`,
      '/docs/application-access.mdx': `---
title: "Application Access"
description: "Guides related to Application Access"
---

{/*TOPICS*/}
`,
      '/docs/application-access/page1.mdx': `---
title: "Application Access Page 1"
description: "Protecting App 1 with Teleport"
---`,
      '/docs/application-access/page2.mdx': `---
title: "Application Access Page 2"
description: "Protecting App 2 with Teleport"
---`,
    });
    const fs = createFsFromVolume(vol);
    const frag = new TopicContentsFragment(fs, '/docs');
    const actual = frag.makeTopicPage();
    expect(actual).toBe(expected);
  });

  test('treats links to directories as regular links (multiple)', () => {
    const expected = `---
title: "Documentation Home"
description: "Guides to setting up the product."
---

Guides to setting up the product.

{/*TOPICS*/}

- [Application Access](docs/application-access.mdx): Guides related to Application Access
- [Database Access](docs/database-access.mdx): Guides related to Database Access.
`;

    const vol = Volume.fromJSON(testFilesTwoSections);
    const fs = createFsFromVolume(vol);
    const frag = new TopicContentsFragment(fs, '/docs');
    const actual = frag.makeTopicPage();
    expect(actual).toBe(expected);
  });

  test('limits menus to one directory level', () => {
    const expected = `---
title: Documentation Home
description: Guides to setting up the product.
---

Guides to setting up the product.

{/*TOPICS*/}

- [Application Access](docs/application-access.mdx): Guides related to Application Access
`;

    const vol = Volume.fromJSON({
      '/docs.mdx': `---
title: Documentation Home
description: Guides to setting up the product.
---

Guides to setting up the product.

{/*TOPICS*/}
`,
      '/docs/application-access.mdx': `---
title: "Application Access"
description: "Guides related to Application Access"
---

{/*TOPICS*/}
`,
      '/docs/application-access/page1.mdx': `---
title: "Application Access Page 1"
description: "Protecting App 1 with Teleport"
---`,
      '/docs/application-access/page2.mdx': `---
title: "Application Access Page 2"
description: "Protecting App 2 with Teleport"
---`,
      '/docs/application-access/jwt.mdx': `---
title: "JWT guides"
description: "Guides related to JWTs"
---`,
      '/docs/application-access/jwt/page1.mdx': `---
title: "JWT Page 1"
description: "Protecting JWT App 1 with Teleport"
---`,
      '/docs/application-access/jwt/page2.mdx': `---
title: "JWT Page 2"
description: "Protecting JWT App 2 with Teleport"
---`,
    });
    const fs = createFsFromVolume(vol);
    const frag = new TopicContentsFragment(fs, '/docs');
    const actual = frag.makeTopicPage();
    expect(actual).toBe(expected);
  });

  test(`throws an error if a root menu page does not have "TOPICS" delimiter`, () => {
    const vol = Volume.fromJSON({
      '/docs.mdx': `---
title: "Documentation Home"
description: "Guides to setting up the product."
`,
      '/docs/application-access.mdx': `---
title: "Application Access"
description: "Guides related to Application Access"
---`,
    });

    const fs = createFsFromVolume(vol);
    expect(() => {
      const frag = new TopicContentsFragment(fs, '/docs');
      frag.makeTopicPage();
    }).toThrow('TOPICS');
  });

  test(`throws an error on a generated menu page that does not correspond to a subdirectory`, () => {
    const vol = Volume.fromJSON({
      '/docs.mdx': `---
title: "Documentation Home"
description: "Guides to setting up the product."
---

{/*TOPICS*/}
`,
      '/docs/application-access.mdx': `---
title: "Application Access"
description: "Guides related to Application Access"
---

{/*TOPICS*/}
`,
      '/docs/application-access/page1.mdx': `---
title: "Application Access Page 1"
description: "Protecting App 1 with Teleport"
---`,
      '/docs/jwt.mdx': `---
title: "JWT guides"
description: "Guides related to JWTs"
---

{/*TOPICS*/}
`,
    });

    const fs = createFsFromVolume(vol);
    expect(() => {
      const frag = new TopicContentsFragment(fs, '/docs');
      frag.makeTopicPage();
    }).toThrow('jwt.mdx');
  });

  test('overwrites topics rather than append to them', () => {
    const expected = `---
title: "Database Access"
description: "Guides related to Database Access."
---

Guides related to Database Access.

{/*TOPICS*/}

- [Database Access Page 1](database-access/page1.mdx): Protecting DB 1 with Teleport
- [Database Access Page 2](database-access/page2.mdx): Protecting DB 2 with Teleport
`;

    const vol = Volume.fromJSON(testFilesTwoSections);
    const fs = createFsFromVolume(vol);
    const frag = new TopicContentsFragment(fs, '/docs/database-access');
    fs.writeFileSync("/docs/database-access.mdx", frag.makeTopicPage());
    const actual = frag.makeTopicPage();
    expect(actual).toBe(expected);
  });

});

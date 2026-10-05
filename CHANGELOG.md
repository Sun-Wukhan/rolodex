# Changelog

## 1.0.0 (2026-10-05)


### Features

* add ABC/XYC identity provider connectors, mock vendors and seed command ([6cc3bde](https://github.com/Sun-Wukhan/rolodex/commit/6cc3bde48b8aafd33f7810e41f07f7e00394014a))
* add auth, profile and identity services with JWT-secured REST API ([4566871](https://github.com/Sun-Wukhan/rolodex/commit/4566871db18a5532019a400e9c305090cd07d0b0))
* add domain model, migrations and postgres/sqlite repositories ([9fcf021](https://github.com/Sun-Wukhan/rolodex/commit/9fcf021343e7e38a47a336d2d3a54fb5fb970201))
* add minikube deployment and one-command local dev runner ([a04dac5](https://github.com/Sun-Wukhan/rolodex/commit/a04dac5a559472b454354138cb79110689cf42d8))
* add minikube deployment and one-command local dev runner ([7b8cc9a](https://github.com/Sun-Wukhan/rolodex/commit/7b8cc9ac90dcdd926a7ffa395243e0b7d4536a3b))
* add optional Google sign-in through Firebase ([39c8353](https://github.com/Sun-Wukhan/rolodex/commit/39c8353bae7cfd678a8c5ca534b2c29a1f5de5db))
* add React frontend with reusable styled-components UI ([e368d3b](https://github.com/Sun-Wukhan/rolodex/commit/e368d3b2e653164497533c4a988084ce887ef7b8))
* add static GitHub Pages demo backed by an in-browser API ([e1e0eb4](https://github.com/Sun-Wukhan/rolodex/commit/e1e0eb422a85671347c89c9c51690907acd3eb45))
* add static GitHub Pages demo backed by an in-browser API ([00acefb](https://github.com/Sun-Wukhan/rolodex/commit/00acefbef1239bcf892f17874275db26980a77fb))
* CI/CD delivery pipeline with SAST, DAST, post-deploy tests and notifications ([db739c0](https://github.com/Sun-Wukhan/rolodex/commit/db739c0bc428c7895a0b17c62e9219d100a3bc4b))
* route profile reads to a streaming replica ([c483cea](https://github.com/Sun-Wukhan/rolodex/commit/c483ceaa1c9183141ea7a7853d2217d5bc6fb5d1))
* route profile reads to a streaming replica ([1e73fa1](https://github.com/Sun-Wukhan/rolodex/commit/1e73fa1a57cbff4d6dc97e0c31973778669f08e1))
* store credentials in a separate database from profiles ([9469341](https://github.com/Sun-Wukhan/rolodex/commit/9469341efa8b2d40ec787e70963b8ce6be5dc0e3))
* store credentials in a separate database from profiles ([ec640eb](https://github.com/Sun-Wukhan/rolodex/commit/ec640eb30d339d56cd1b10298fdba9f6440575d7))


### Bug Fixes

* **api:** reject control characters and invalid UTF-8 before they reach the datastore ([23e07ac](https://github.com/Sun-Wukhan/rolodex/commit/23e07accd31c35d7d2820435d6df5179ab02929e))
* **deps:** hold TypeScript to the range typescript-eslint supports ([efb809d](https://github.com/Sun-Wukhan/rolodex/commit/efb809d39640c2f6212aee5bb01e772fd14c2de4))
* harden client IP trust model and resolve static analysis findings ([aa667a0](https://github.com/Sun-Wukhan/rolodex/commit/aa667a0f1b86b6151aa32a997142e1a821e29d61))
* satisfy revive on the AnyDomain comment ([dd44294](https://github.com/Sun-Wukhan/rolodex/commit/dd44294e7756c9dc116b3d4ed686d07eed798570))
* satisfy revive on the AnyDomain comment ([75cdce6](https://github.com/Sun-Wukhan/rolodex/commit/75cdce641f693b2acaf485f67a9826d262713c1a))
* serialise migrations with an advisory lock and use a numeric non-root UID ([5684150](https://github.com/Sun-Wukhan/rolodex/commit/56841505433edbfa0603fe2ac3e9f7d2d192ec5f))
* **web:** send security headers on every response and harden containers ([df1c136](https://github.com/Sun-Wukhan/rolodex/commit/df1c1361c6d85cf95d1489afd5c1c05ff620dec9))

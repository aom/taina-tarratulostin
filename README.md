# tarra

Pieni komentorivityökalu, joka tulostaa tekstiä ja ruuvisymboleja **DYMO LabelManager 280** -tulostimella macOS:lla. Tarkoitus on tarroittaa esimerkiksi ruuvilaatikoita: `M3x8` + ristipääsymboli, `M4x20` + kuusiokolo ja niin edelleen.

Työkalu ei tarvitse Dymon omaa ohjelmaa tulostaessaan. Se luo PDF:n ja lähettää sen macOS:n tulostusjärjestelmälle (CUPS) `lp`-komennolla.

```sh
tarra "M3x8 :ph:"
tarra "M4x20|A2 ruostumaton :hex:"
```

## Mitä tarvitset

- Mac ja DYMO LabelManager 280 USB-kaapelilla
- 12 mm:n D1-teippi
- DYMO:n tulostinajuri asennettuna ja hyväksyttynä (katso alla)
- [Go](https://go.dev/dl/) 1.21 tai uudempi käännöstä varten (`brew install go`)

### Ajurin asennus (kerran)

1. Asenna **DYMO Connect** Dymon sivuilta (dymo.com → Support → Downloads).
2. Jos ohjelma sanoo *System Extension Blocked*, hyväksy laajennus: Järjestelmäasetukset → Tietosuoja ja turvallisuus (ja/tai Yleiset → Kirjautumiskohteet ja laajennukset). Uudelleenkäynnistys voi olla tarpeen.
3. Kytke tulostin ja varmista, että **DYMO LabelManager 280** näkyy kohdassa Järjestelmäasetukset → Tulostimet ja skannerit tilassa *Idle*.

Dymo Connectia ei tarvitse pitää auki, mutta ajuri tulee sen asennuksen mukana, joten älä poista sitä.

## Asennus

```sh
cd tarra
go build -o tarra .
sudo mv tarra /usr/local/bin/      # valinnainen, jotta komento toimii kaikkialla
```

Jos lataat tiedostot selaimella, tarvitset sekä `main.go`:n että `go.mod`:n samaan kansioon.

## Käyttö

Valinnat kirjoitetaan **ennen** tekstiä.

```sh
tarra "teksti"                      # yksi tarra
tarra "ylärivi|alarivi"             # kaksi riviä (| = rivinvaihto)
tarra "M3x8 :ph:" "M4x20 :hex:"     # kaksi tarraa, kumpikin omana tulostustyönään
tarra -c 3 "M3x8 :ph:"              # kolme kopiota
tarra -n "M3x8 :ph:"                # vain PDF, avautuu Esikatseluun, ei tulosta
tarra -list                         # listaa symbolit
```

**Kokeile aina uusi asettelu ensin `-n`-valinnalla**, ettei teippiä mene hukkaan. Esikatselussa tarra näkyy pystysuuntaisena kapeana sivuna, jossa teksti on kyljellään. Se on tarkoituksellista: ajuri haluaa pystysivun.

### Symbolit

Kirjoita symbolin nimi kaksoispisteiden väliin tekstin joukkoon, esim. `M3x8 :ph:`.

| Symboli | Nimi | Aliakset |
|---|---|---|
| Ristipää (Phillips) | `:ph:` | `:risti:` |
| Pozidriv | `:pz:` | `:pozi:` |
| Ura | `:slot:` | `:ura:` |
| Kuusiokolo | `:hex:` | `:kuusio:` |
| Torx | `:torx:` | |
| Mutteri | `:nut:` | `:mutteri:` |
| Aluslevy | `:washer:` | `:levy:` |
| Kupukanta (sivukuva) | `:pan:` | `:kupu:` |
| Uppokanta (sivukuva) | `:csk:` | `:uppo:` |

Tuntematon `:nimi:` tulostuu tavallisena tekstinä.

### Tarran koko ja asettelu

- Tarra on **tekstin mittainen, enintään 42 mm**. Liian pitkä teksti pienennetään niin, että se mahtuu.
- Teksti alkaa vasemmasta reunasta. Teksti on oletuksena lihavoitua.
- Tarran korkeus on 12 mm teippiä varten. Kahdella rivillä fontti pienenee.

### Valinnat

| Valinta | Oletus | Selitys |
|---|---|---|
| `-n` | pois | Älä tulosta, luo vain PDF ja avaa se Esikatselussa |
| `-c N` | 1 | Kopioiden määrä |
| `-max MM` | 42 | Tarran enimmäispituus |
| `-len MM` | 0 | Kiinteä pituus (0 = tekstin mittainen) |
| `-min MM` | 0 | Tarran vähimmäispituus |
| `-pad MM` | 1.5 | Marginaali vasemmalla ja oikealla |
| `-vmargin MM` | 1 | Marginaali ylä- ja alareunassa |
| `-voffset MM` | 2.4 | Siirtää sisältöä alaspäin (negatiivinen = ylöspäin) |
| `-align` | left | `left`, `center` tai `right` (vaikuttaa vain kiinteällä `-len`) |
| `-regular` | pois | Normaali lihavoidun sijaan |
| `-rot` | 270 | Sivun kierto: 270, 90 tai 0 |
| `-printer NIMI` | etsii DYMO:n | CUPS-tulostimen nimi (`lpstat -p`) |
| `-media` | auto | `auto`, `none` tai ajurin kokonimi, esim. `w35h144` |
| `-o ASETUS` | | Lisäasetus `lp`:lle, toistettava, esim. `-o DymoHalftoning=NLL` |
| `-pdf POLKU` | | Tallenna PDF myös tiedostoon |

## Vianmääritys

| Oire | Kokeile |
|---|---|
| Tarra tulee tyhjänä | Tarkista, että `lpoptions -p DYMO_LabelManager_280 -l` näyttää `w35h…`-kokoja. Kokeile `-media none -o DymoContinuousPaper=0` tai `-media w35h144` |
| Teksti ylösalaisin tai väärästä päästä | `-rot 90` |
| Sisältö liian ylhäällä tai alhaalla | Säädä `-voffset` (esim. 2.0 tai 2.8) |
| Ylä- tai alareuna leikkautuu | Kasvata `-vmargin` (esim. 1.5) |
| Ohuet viivat rakoilevat | `-o DymoHalftoning=Default` tai `-o DymoHalftoning=NLL` |
| `DYMO-tulostinta ei löytynyt` | Anna nimi: `-printer DYMO_LabelManager_280` |
| Tulostin ei näy Macin asetuksissa | Hyväksy DYMO:n järjestelmälaajennus (ks. yllä), kokeile toista USB-porttia ilman hubia |
| Tarran alussa on tyhjää (noin 8 mm) | Näyttää tulostimen omalta syöttömarginaalilta, ei työkalun viralta. Leikkaa ylimääräinen pois |

## Rajoitukset

- Tulostin leikkaa tarrat käsin: tulostuksen jälkeen leikkaa tarra tulostimen leikkurilla.
- Tekstin leveys lasketaan Helvetican leveystaulukolla, joten skandit ja erikoismerkit ovat likimääräisiä. Teksti on aina ASCII + Latin-1 + `€`.
- Oletusarvot (`-voffset`, `-rot`) on viritetty yhdelle LabelManager 280:lle ja 12 mm:n teipille. Muilla teippileveyksillä ajurin paperikoot (`w18…`, `w26…`) ovat eri ja `-tape` pitää säätää.
- Ei Windows- tai Linux-tukea: tulostus nojaa `lp`/`lpstat`-komentoihin (toimivat periaatteessa myös Linuxilla CUPS:n kanssa, mutta ei testattu).

## Lisää oma symboli

Symbolit ovat `symbols`-taulukossa tiedostossa `main.go`. Jokainen symboli on funktio, joka piirtää neliöön `(x, y)`, koko `s`, PDF-piirtokomennoilla (viivat, täytöt, ympyrät). Lisää rivi taulukkoon nimillä ja piirtofunktiolla, käännä uudelleen ja tarkista tulos `-n`-valinnalla.

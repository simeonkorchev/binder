import { parseCardCode } from './parseCardCode'

describe('parseCardCode', () => {
  it('carries the prefix, the region and the serial through unchanged', () => {
    // Every part holds a different value, so a part that was dropped, swapped
    // or re-cased changes the result (000-principles.md section 9).
    expect(parseCardCode('MP21-FR123')).toBe('MP21-FR123')
  })

  it.each([
    ['the original English run, no region letters', 'SDK-001', 'SDK-001'],
    ['a three-letter prefix', 'LOB-EN001', 'LOB-EN001'],
    ['a prefix ending in digits', 'MP21-EN123', 'MP21-EN123'],
    ['a prefix starting with digits', '15AX-EN001', '15AX-EN001'],
    ['a four-letter prefix', 'BLAR-EN001', 'BLAR-EN001'],
    ['a five-digit serial', 'LOB-EN00123', 'LOB-EN00123'],
  ])('reads %s', (_case, ocrText, expected) => {
    expect(parseCardCode(ocrText)).toBe(expected)
  })

  it.each([
    ['lower case', 'lob-en001'],
    ['mixed case', 'Lob-En001'],
    ['an en dash', 'LOB–EN001'],
    ['an em dash', 'LOB—EN001'],
    ['a minus sign', 'LOB−EN001'],
    ['a non-breaking hyphen', 'LOB‑EN001'],
    ['surrounding brackets', '(LOB-EN001)'],
    ['a trailing full stop', 'LOB-EN001.'],
    ['surrounding quotes', '"LOB-EN001"'],
    ['a leading bullet', '•LOB-EN001'],
    ['surrounding whitespace', '   LOB-EN001\n'],
    ['a zero read as the letter O', 'LOB-ENO01'],
    ['every zero read as the letter O', 'LOB-ENOO1'],
    ['a one read as the letter I', 'LOB-EN0OI'],
    ['a one read as a lower-case l', 'lob-enool'],
    ['the whole serial read as letters', 'LOB-ENOOl'],
  ])('reads a code through %s', (_case, ocrText) => {
    expect(parseCardCode(ocrText)).toBe('LOB-EN001')
  })

  it.each([
    ['a five read as the letter S', 'LOB-EN00S', 'LOB-EN005'],
    ['an eight read as the letter B', 'LOB-EN0B1', 'LOB-EN081'],
  ])('repairs %s', (_case, ocrText, expected) => {
    expect(parseCardCode(ocrText)).toBe(expected)
  })

  it('finds the code in everything else ML Kit read off the card', () => {
    const cardFace = [
      'Blue-Eyes White Dragon',
      '[Dragon / Normal]',
      'This legendary dragon is a powerful engine of destruction.',
      'ATK/3000 DEF/2500',
      'LOB-EN001',
      '89631139',
      '©1996 KAZUKI TAKAHASHI',
    ].join('\n')

    expect(parseCardCode(cardFace)).toBe('LOB-EN001')
  })

  it('returns the first code when a sweep caught two cards at once', () => {
    expect(parseCardCode('LOB-EN001\nSDM-EN002')).toBe('LOB-EN001')
  })

  it.each([
    ['nothing was read', ''],
    ['only whitespace was read', '  \n\t '],
    ["the monster's stat line, which has the shape of a code", 'ATK/2500 DEF/2100'],
    ['a hyphenated card name', 'Blue-Eyes White Dragon'],
    ['the eight-digit passcode', '89631139'],
    ['a serial with no prefix', '001'],
    ['a printed year range', '1996-2024'],
    ['a prefix with no letter in it', '2024-EN001'],
    ['a serial too short to be one', 'LOB-EN01'],
    ['a serial too long to be one', 'LOB-EN001234'],
    ['a prefix too long to be one', 'LONGSET-EN001'],
    ['a second separator', 'LOB-EN-001'],
    // The region letters are not repaired: a digit standing in for one leaves
    // nothing that can be matched on, and the server's name rung is what
    // rescues the card (spec.md US2).
    ['a region letter read as a digit', 'LOB-3N001'],
    // A dropped separator leaves one token, and any word on the card is one.
    ['a separator OCR dropped entirely', 'LOBEN001'],
  ])('finds no code when %s', (_case, ocrText) => {
    expect(parseCardCode(ocrText)).toBeNull()
  })
})

import type { ParsedSseEvent } from '@/types/sse';

export function parseSseChunk(buffer: string): { events: ParsedSseEvent[]; rest: string } {
  const parts = buffer.split('\n\n');
  const rest = parts.pop() ?? '';
  const events = parts.map(parseSseBlock).filter((item): item is ParsedSseEvent => Boolean(item));
  return { events, rest };
}

function parseSseBlock(block: string): ParsedSseEvent | null {
  const lines = block.split('\n');
  const event = lines.find((line) => line.startsWith('event:'))?.slice(6).trim() ?? 'message';
  const dataText = lines
    .filter((line) => line.startsWith('data:'))
    .map((line) => line.slice(5).trim())
    .join('\n');
  if (!dataText) return null;
  try {
    return { event, data: JSON.parse(dataText) };
  } catch {
    return { event, data: dataText };
  }
}

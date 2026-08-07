import { LiveApiError } from "@/live-api/api-client";
import type { OperationDocument, OperationDocumentType, OperationPage } from "@/live-api/types";

const DEFAULT_MAX_PAGES = 50;
const DEFAULT_MAX_ITEMS = 5_000;

export interface OperationPageReader {
  listOperationDocuments(type?: OperationDocumentType, cursor?: string): Promise<OperationPage>;
}

interface PaginationLimits {
  maxPages?: number;
  maxItems?: number;
}

function paginationError(type: OperationDocumentType, detail: string): LiveApiError {
  return new LiveApiError({
    type: "urn:itemba-z:control-center:operation-pagination",
    title: "Operation documents unavailable",
    status: 503,
    code: "operation_document_pagination_invalid",
    detail: `${detail} Document type: ${type}.`,
  });
}

export async function listAllOperationDocuments(
  reader: OperationPageReader,
  type: OperationDocumentType,
  limits: PaginationLimits = {},
): Promise<OperationDocument[]> {
  const maxPages = limits.maxPages ?? DEFAULT_MAX_PAGES;
  const maxItems = limits.maxItems ?? DEFAULT_MAX_ITEMS;
  if (!Number.isSafeInteger(maxPages) || maxPages < 1 || !Number.isSafeInteger(maxItems) || maxItems < 1) {
    throw paginationError(type, "The document pagination safety limits are invalid.");
  }

  const documents: OperationDocument[] = [];
  const documentIds = new Set<string>();
  const cursors = new Set<string>();
  let cursor: string | undefined;

  for (let pageIndex = 0; pageIndex < maxPages; pageIndex += 1) {
    const page = await reader.listOperationDocuments(type, cursor);
    for (const document of page.items) {
      if (documentIds.has(document.id)) continue;
      documentIds.add(document.id);
      documents.push(document);
      if (documents.length > maxItems) {
        throw paginationError(type, `The bounded read exceeded ${maxItems} documents.`);
      }
    }

    const nextCursor = page.next_cursor?.trim();
    if (!nextCursor) return documents;
    if (cursors.has(nextCursor) || nextCursor === cursor) {
      throw paginationError(type, "The live ERP returned a repeated document cursor.");
    }
    cursors.add(nextCursor);
    cursor = nextCursor;
  }

  throw paginationError(type, `The bounded read exceeded ${maxPages} pages.`);
}

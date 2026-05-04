"use client";

import * as React from "react";
import {
  ColumnDef,
  SortingState,
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
} from "@tanstack/react-table";
import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from "lucide-react";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useTranslation } from "@/lib/i18n";

// Standalone debounced search input - completely isolated from table re-renders
const DebouncedInput = React.memo(function DebouncedInput({
  placeholder,
  onChange,
  delay = 400,
}: {
  placeholder: string;
  onChange: (value: string) => void;
  delay?: number;
}) {
  const [value, setValue] = React.useState("");
  const timerRef = React.useRef<any>(null);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const v = e.target.value;
    setValue(v);
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => onChange(v), delay);
  };

  return (
    <input
      type="text"
      placeholder={placeholder}
      value={value}
      onChange={handleChange}
      className="bg-white/5 border border-white/10 rounded-lg px-3 py-1.5 text-sm text-white placeholder-white/30 w-72 focus:outline-none focus:border-oklavier-blue/50 transition-colors"
    />
  );
});

interface DataTableProps<TData, TValue> {
  columns: ColumnDef<TData, TValue>[];
  data: TData[];
  searchPlaceholder?: string;
  pageSize?: number;
  toolbar?: React.ReactNode;
  emptyMessage?: string;
  manualPagination?: boolean;
  pageCount?: number;
  page?: number;
  onPageChange?: (page: number) => void;
  total?: number;
  onSearchChange?: (search: string) => void;
}

export function DataTable<TData, TValue>({
  columns,
  data,
  searchPlaceholder,
  pageSize = 20,
  toolbar,
  emptyMessage,
  manualPagination = false,
  pageCount,
  page,
  onPageChange,
  total,
  onSearchChange,
}: DataTableProps<TData, TValue>) {
  const { t } = useTranslation();
  const [sorting, setSorting] = React.useState<SortingState>([]);
  // Client-side search only used when NOT in manualPagination mode
  const [globalFilter, setGlobalFilter] = React.useState("");

  const table = useReactTable({
    data,
    columns,
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    // Client-side features only when not server-side
    ...(!manualPagination ? {
      onGlobalFilterChange: setGlobalFilter,
      getPaginationRowModel: getPaginationRowModel(),
      getFilteredRowModel: getFilteredRowModel(),
      state: { sorting, globalFilter },
      initialState: { pagination: { pageSize } },
    } : {
      manualPagination: true,
      pageCount: pageCount || 1,
      state: {
        sorting,
        pagination: { pageIndex: (page || 1) - 1, pageSize },
      },
    }),
  });

  // Pagination helpers
  const isServer = manualPagination && !!onPageChange;
  const currentPage = isServer ? (page || 1) : table.getState().pagination.pageIndex + 1;
  const totalPageCount = isServer ? (pageCount || 1) : table.getPageCount();
  const canPrev = isServer ? (page || 1) > 1 : table.getCanPreviousPage();
  const canNext = isServer ? (page || 1) < (pageCount || 1) : table.getCanNextPage();
  const goFirst = () => isServer ? onPageChange!(1) : table.setPageIndex(0);
  const goPrev = () => isServer ? onPageChange!((page || 1) - 1) : table.previousPage();
  const goNext = () => isServer ? onPageChange!((page || 1) + 1) : table.nextPage();
  const goLast = () => isServer ? onPageChange!(pageCount || 1) : table.setPageIndex(table.getPageCount() - 1);
  const totalResults = total != null ? total : table.getFilteredRowModel().rows.length;

  return (
    <div>
      {/* Toolbar */}
      <div className="flex items-center justify-between gap-3 mb-4">
        {manualPagination && onSearchChange ? (
          // Server-side: standalone debounced input, NOT connected to TanStack
          <DebouncedInput
            placeholder={searchPlaceholder || t("common.search")}
            onChange={onSearchChange}
          />
        ) : (
          // Client-side: directly connected to TanStack globalFilter
          <input
            type="text"
            placeholder={searchPlaceholder || t("common.search")}
            value={globalFilter}
            onChange={(e) => setGlobalFilter(e.target.value)}
            className="bg-white/5 border border-white/10 rounded-lg px-3 py-1.5 text-sm text-white placeholder-white/30 w-72 focus:outline-none focus:border-oklavier-blue/50 transition-colors"
          />
        )}
        {toolbar}
      </div>

      {/* Table */}
      <div className="bg-white/5 rounded-xl border border-white/10 overflow-hidden">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id} className="border-b border-white/10 hover:bg-transparent">
                {headerGroup.headers.map((header) => (
                  <TableHead key={header.id} className="text-white/40 text-xs uppercase tracking-wider font-semibold px-4 py-3">
                    {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows?.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow key={row.id} className="border-b border-white/5 hover:bg-white/5 transition-colors">
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id} className="px-4 py-2.5 text-white/80 text-sm">
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell colSpan={columns.length} className="h-24 text-center text-white/30">
                  {emptyMessage || t("common.no_results")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      {/* Pagination */}
      {totalPageCount > 1 && (
        <div className="flex items-center justify-between mt-4">
          <p className="text-white/30 text-xs">{totalResults} {t("common.results")}</p>
          <div className="flex items-center gap-1">
            <button onClick={goFirst} disabled={!canPrev} className="p-1.5 rounded-lg text-white/40 hover:text-white hover:bg-white/10 disabled:opacity-20 disabled:cursor-not-allowed transition-colors">
              <ChevronsLeft className="size-4" />
            </button>
            <button onClick={goPrev} disabled={!canPrev} className="p-1.5 rounded-lg text-white/40 hover:text-white hover:bg-white/10 disabled:opacity-20 disabled:cursor-not-allowed transition-colors">
              <ChevronLeft className="size-4" />
            </button>
            <span className="px-3 text-sm text-white/50">{currentPage} / {totalPageCount}</span>
            <button onClick={goNext} disabled={!canNext} className="p-1.5 rounded-lg text-white/40 hover:text-white hover:bg-white/10 disabled:opacity-20 disabled:cursor-not-allowed transition-colors">
              <ChevronRight className="size-4" />
            </button>
            <button onClick={goLast} disabled={!canNext} className="p-1.5 rounded-lg text-white/40 hover:text-white hover:bg-white/10 disabled:opacity-20 disabled:cursor-not-allowed transition-colors">
              <ChevronsRight className="size-4" />
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

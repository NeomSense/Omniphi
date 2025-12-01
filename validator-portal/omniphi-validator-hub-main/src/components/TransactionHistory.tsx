import { useState, useEffect } from 'react';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  ArrowUpRight,
  ArrowDownLeft,
  RefreshCw,
  ExternalLink,
  Search,
  Filter,
  Calendar,
  Loader2,
} from 'lucide-react';
import { useValidatorStore } from '@/store/validatorStore';
import { toast } from 'sonner';

interface Transaction {
  hash: string;
  type: 'delegate' | 'undelegate' | 'redelegate' | 'claim_rewards' | 'create_validator' | 'edit_validator';
  timestamp: string;
  status: 'success' | 'failed' | 'pending';
  amount?: string;
  fee: string;
  height: number;
  from?: string;
  to?: string;
  memo?: string;
}

export const TransactionHistory = () => {
  const { walletAddress } = useValidatorStore();
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [loading, setLoading] = useState(false);
  const [filter, setFilter] = useState<string>('all');
  const [searchTerm, setSearchTerm] = useState('');
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const ITEMS_PER_PAGE = 20;

  useEffect(() => {
    if (walletAddress) {
      fetchTransactions();
    }
  }, [walletAddress, page, filter]);

  const fetchTransactions = async () => {
    setLoading(true);
    try {
      // TODO: Implement actual RPC/API call
      // const response = await fetch(`${API_URL}/cosmos/tx/v1beta1/txs?events=message.sender='${walletAddress}'&pagination.offset=${(page - 1) * ITEMS_PER_PAGE}&pagination.limit=${ITEMS_PER_PAGE}`);

      // Mock data for now
      const mockTxs: Transaction[] = [
        {
          hash: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
          type: 'delegate',
          timestamp: new Date(Date.now() - 3600000).toISOString(),
          status: 'success',
          amount: '1000.000000',
          fee: '0.005000',
          height: 12345678,
          to: 'omniphivaloper1abc...',
        },
        {
          hash: '0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890',
          type: 'claim_rewards',
          timestamp: new Date(Date.now() - 86400000).toISOString(),
          status: 'success',
          amount: '12.345678',
          fee: '0.003000',
          height: 12344000,
        },
        {
          hash: '0x7890abcdef1234567890abcdef1234567890abcdef1234567890abcdef123456',
          type: 'undelegate',
          timestamp: new Date(Date.now() - 172800000).toISOString(),
          status: 'success',
          amount: '500.000000',
          fee: '0.005000',
          height: 12340000,
          from: 'omniphivaloper1xyz...',
        },
        {
          hash: '0xdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abc',
          type: 'create_validator',
          timestamp: new Date(Date.now() - 259200000).toISOString(),
          status: 'success',
          amount: '10000.000000',
          fee: '0.010000',
          height: 12335000,
          memo: 'My new validator',
        },
      ];

      setTransactions(mockTxs);
      setTotalPages(3); // Mock pagination
    } catch (error) {
      console.error('Failed to fetch transactions:', error);
      toast.error('Failed to load transactions');
    } finally {
      setLoading(false);
    }
  };

  const getTypeIcon = (type: Transaction['type']) => {
    const icons = {
      delegate: <ArrowUpRight className="h-4 w-4 text-green-500" />,
      undelegate: <ArrowDownLeft className="h-4 w-4 text-red-500" />,
      redelegate: <RefreshCw className="h-4 w-4 text-blue-500" />,
      claim_rewards: <ArrowDownLeft className="h-4 w-4 text-yellow-500" />,
      create_validator: <ArrowUpRight className="h-4 w-4 text-purple-500" />,
      edit_validator: <RefreshCw className="h-4 w-4 text-accent" />,
    };
    return icons[type];
  };

  const getTypeLabel = (type: Transaction['type']) => {
    const labels = {
      delegate: 'Delegate',
      undelegate: 'Undelegate',
      redelegate: 'Redelegate',
      claim_rewards: 'Claim Rewards',
      create_validator: 'Create Validator',
      edit_validator: 'Edit Validator',
    };
    return labels[type];
  };

  const getStatusBadge = (status: Transaction['status']) => {
    const variants: Record<Transaction['status'], 'default' | 'secondary' | 'destructive'> = {
      success: 'default',
      pending: 'secondary',
      failed: 'destructive',
    };
    return <Badge variant={variants[status]}>{status}</Badge>;
  };

  const formatAmount = (amount: string) => {
    return parseFloat(amount).toLocaleString(undefined, {
      minimumFractionDigits: 2,
      maximumFractionDigits: 6,
    });
  };

  const formatDate = (timestamp: string) => {
    const date = new Date(timestamp);
    const now = new Date();
    const diff = now.getTime() - date.getTime();
    const hours = Math.floor(diff / 3600000);
    const days = Math.floor(diff / 86400000);

    if (hours < 1) return 'Just now';
    if (hours < 24) return `${hours}h ago`;
    if (days < 7) return `${days}d ago`;
    return date.toLocaleDateString();
  };

  const filteredTransactions = transactions.filter((tx) => {
    const matchesFilter = filter === 'all' || tx.type === filter;
    const matchesSearch =
      searchTerm === '' ||
      tx.hash.toLowerCase().includes(searchTerm.toLowerCase()) ||
      getTypeLabel(tx.type).toLowerCase().includes(searchTerm.toLowerCase());
    return matchesFilter && matchesSearch;
  });

  const exportToCsv = () => {
    const headers = ['Hash', 'Type', 'Date', 'Amount', 'Fee', 'Status', 'Height'];
    const rows = filteredTransactions.map((tx) => [
      tx.hash,
      getTypeLabel(tx.type),
      new Date(tx.timestamp).toISOString(),
      tx.amount || 'N/A',
      tx.fee,
      tx.status,
      tx.height.toString(),
    ]);

    const csvContent = [
      headers.join(','),
      ...rows.map((row) => row.join(',')),
    ].join('\n');

    const blob = new Blob([csvContent], { type: 'text/csv' });
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `omniphi-transactions-${Date.now()}.csv`;
    a.click();
    window.URL.revokeObjectURL(url);

    toast.success('Transactions exported to CSV');
  };

  if (!walletAddress) {
    return (
      <Card className="glass-card p-8 text-center">
        <p className="text-muted-foreground">Please connect your wallet to view transaction history</p>
      </Card>
    );
  }

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <h2 className="text-3xl font-bold">Transaction History</h2>
          <p className="text-muted-foreground">View all your blockchain transactions</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={exportToCsv}>
            Export CSV
          </Button>
          <Button onClick={fetchTransactions} disabled={loading}>
            {loading ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
            Refresh
          </Button>
        </div>
      </div>

      {/* Summary Stats */}
      <div className="grid md:grid-cols-4 gap-4">
        <Card className="glass-card p-4">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-primary/10">
              <ArrowUpRight className="h-5 w-5 text-primary" />
            </div>
            <div>
              <p className="text-2xl font-bold">{transactions.length}</p>
              <p className="text-xs text-muted-foreground">Total Transactions</p>
            </div>
          </div>
        </Card>

        <Card className="glass-card p-4">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-green-500/10">
              <ArrowUpRight className="h-5 w-5 text-green-500" />
            </div>
            <div>
              <p className="text-2xl font-bold">
                {transactions.filter((tx) => tx.type === 'delegate').length}
              </p>
              <p className="text-xs text-muted-foreground">Delegations</p>
            </div>
          </div>
        </Card>

        <Card className="glass-card p-4">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-yellow-500/10">
              <ArrowDownLeft className="h-5 w-5 text-yellow-500" />
            </div>
            <div>
              <p className="text-2xl font-bold">
                {transactions.filter((tx) => tx.type === 'claim_rewards').length}
              </p>
              <p className="text-xs text-muted-foreground">Rewards Claimed</p>
            </div>
          </div>
        </Card>

        <Card className="glass-card p-4">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-blue-500/10">
              <Calendar className="h-5 w-5 text-blue-500" />
            </div>
            <div>
              <p className="text-2xl font-bold">
                {transactions.filter((tx) => {
                  const txDate = new Date(tx.timestamp);
                  const now = new Date();
                  const diff = now.getTime() - txDate.getTime();
                  return diff < 86400000 * 7; // Last 7 days
                }).length}
              </p>
              <p className="text-xs text-muted-foreground">This Week</p>
            </div>
          </div>
        </Card>
      </div>

      {/* Filters */}
      <div className="flex flex-col md:flex-row gap-4">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search by hash or type..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="pl-10"
          />
        </div>
        <Select value={filter} onValueChange={setFilter}>
          <SelectTrigger className="w-full md:w-[200px]">
            <Filter className="mr-2 h-4 w-4" />
            <SelectValue placeholder="Filter by type" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Types</SelectItem>
            <SelectItem value="delegate">Delegate</SelectItem>
            <SelectItem value="undelegate">Undelegate</SelectItem>
            <SelectItem value="redelegate">Redelegate</SelectItem>
            <SelectItem value="claim_rewards">Claim Rewards</SelectItem>
            <SelectItem value="create_validator">Create Validator</SelectItem>
            <SelectItem value="edit_validator">Edit Validator</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* Transactions Table */}
      <Card className="glass-card">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Type</TableHead>
              <TableHead>Transaction Hash</TableHead>
              <TableHead>Date</TableHead>
              <TableHead>Amount</TableHead>
              <TableHead>Fee</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="text-right">Action</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={7} className="text-center py-8">
                  <Loader2 className="mx-auto h-8 w-8 animate-spin text-primary" />
                  <p className="text-sm text-muted-foreground mt-2">Loading transactions...</p>
                </TableCell>
              </TableRow>
            ) : filteredTransactions.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} className="text-center py-8 text-muted-foreground">
                  {searchTerm || filter !== 'all' ? 'No transactions found matching your filters' : 'No transactions yet'}
                </TableCell>
              </TableRow>
            ) : (
              filteredTransactions.map((tx) => (
                <TableRow key={tx.hash}>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      {getTypeIcon(tx.type)}
                      <span className="font-medium">{getTypeLabel(tx.type)}</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <code className="text-xs font-mono">
                      {tx.hash.slice(0, 10)}...{tx.hash.slice(-8)}
                    </code>
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-col">
                      <span>{formatDate(tx.timestamp)}</span>
                      <span className="text-xs text-muted-foreground">
                        Block {tx.height.toLocaleString()}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell>
                    {tx.amount ? (
                      <div className="flex flex-col">
                        <span className="font-medium">{formatAmount(tx.amount)}</span>
                        <span className="text-xs text-muted-foreground">OMNI</span>
                      </div>
                    ) : (
                      <span className="text-muted-foreground">N/A</span>
                    )}
                  </TableCell>
                  <TableCell>
                    <span className="text-sm">{formatAmount(tx.fee)} OMNI</span>
                  </TableCell>
                  <TableCell>{getStatusBadge(tx.status)}</TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => {
                        // Open block explorer
                        window.open(`https://explorer.omniphi.xyz/tx/${tx.hash}`, '_blank');
                      }}
                    >
                      <ExternalLink className="h-3 w-3" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </Card>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            Page {page} of {totalPages}
          </p>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page === 1 || loading}
            >
              Previous
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              disabled={page === totalPages || loading}
            >
              Next
            </Button>
          </div>
        </div>
      )}
    </div>
  );
};

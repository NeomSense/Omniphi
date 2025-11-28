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
  Search,
  ExternalLink,
  CheckCircle2,
  AlertCircle,
  TrendingUp,
  Users,
  Award,
} from 'lucide-react';
import { useValidatorStore } from '@/store/validatorStore';
import { validatorApi } from '@/lib/api';
import { toast } from 'sonner';
import { LoadingSpinner } from './Common/LoadingSpinner';
import { ErrorBanner } from './Common/ErrorBanner';

interface ValidatorListItem {
  setupRequest: {
    id: string;
    status: string;
    validatorName: string;
    runMode: string;
    consensusPubkey?: string;
  };
  node?: {
    status: string;
    rpcEndpoint: string;
  };
  chainInfo?: {
    operatorAddress: string;
    jailed: boolean;
    status: string;
    tokens: string;
    votingPower: string;
    commission: {
      commissionRates: {
        rate: string;
      };
    };
  };
  heartbeat?: {
    blockHeight: number;
    lastSeen: string;
  };
}

export const ValidatorList = () => {
  const { walletAddress } = useValidatorStore();
  const [validators, setValidators] = useState<ValidatorListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchTerm, setSearchTerm] = useState('');

  useEffect(() => {
    if (walletAddress) {
      fetchValidators();
    }
  }, [walletAddress]);

  const fetchValidators = async () => {
    if (!walletAddress) return;

    setLoading(true);
    setError(null);

    try {
      const data = await validatorApi.getValidatorsByWallet(walletAddress);
      setValidators(data);
    } catch (err: any) {
      const errorMessage = err?.response?.data?.detail || err?.message || 'Failed to load validators';
      setError(errorMessage);
      toast.error(errorMessage);
    } finally {
      setLoading(false);
    }
  };

  const getStatusBadge = (status: string) => {
    const statusMap: Record<string, { variant: 'default' | 'secondary' | 'destructive' | 'outline'; label: string }> = {
      'pending': { variant: 'secondary', label: 'Pending' },
      'provisioning': { variant: 'secondary', label: 'Provisioning' },
      'ready_for_chain_tx': { variant: 'outline', label: 'Ready' },
      'active': { variant: 'default', label: 'Active' },
      'failed': { variant: 'destructive', label: 'Failed' },
    };

    const config = statusMap[status] || { variant: 'outline' as const, label: status };
    return <Badge variant={config.variant}>{config.label}</Badge>;
  };

  const formatTokens = (tokens: string) => {
    const amount = parseFloat(tokens) / 1_000_000; // Assuming 6 decimals
    return amount.toLocaleString(undefined, { maximumFractionDigits: 2 });
  };

  const filteredValidators = validators.filter(v =>
    v.setupRequest.validatorName.toLowerCase().includes(searchTerm.toLowerCase())
  );

  if (!walletAddress) {
    return (
      <Card className="glass-card p-8 text-center">
        <p className="text-muted-foreground">Please connect your wallet to view validators</p>
      </Card>
    );
  }

  if (loading) {
    return <LoadingSpinner size="lg" text="Loading validators..." className="min-h-[400px]" />;
  }

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <h2 className="text-3xl font-bold">My Validators</h2>
          <p className="text-muted-foreground">Manage all your validators in one place</p>
        </div>
        <Button onClick={fetchValidators} disabled={loading}>
          Refresh
        </Button>
      </div>

      {error && <ErrorBanner message={error} />}

      {/* Summary Cards */}
      <div className="grid md:grid-cols-4 gap-4">
        <Card className="glass-card p-4">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-primary/10">
              <CheckCircle2 className="h-5 w-5 text-primary" />
            </div>
            <div>
              <p className="text-2xl font-bold">{validators.length}</p>
              <p className="text-xs text-muted-foreground">Total Validators</p>
            </div>
          </div>
        </Card>

        <Card className="glass-card p-4">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-green-500/10">
              <Award className="h-5 w-5 text-green-500" />
            </div>
            <div>
              <p className="text-2xl font-bold">
                {validators.filter(v => v.chainInfo?.status === 'BOND_STATUS_BONDED').length}
              </p>
              <p className="text-xs text-muted-foreground">Active</p>
            </div>
          </div>
        </Card>

        <Card className="glass-card p-4">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-yellow-500/10">
              <AlertCircle className="h-5 w-5 text-yellow-500" />
            </div>
            <div>
              <p className="text-2xl font-bold">
                {validators.filter(v => v.setupRequest.status === 'provisioning').length}
              </p>
              <p className="text-xs text-muted-foreground">Provisioning</p>
            </div>
          </div>
        </Card>

        <Card className="glass-card p-4">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-blue-500/10">
              <TrendingUp className="h-5 w-5 text-blue-500" />
            </div>
            <div>
              <p className="text-2xl font-bold">
                {validators.reduce((sum, v) => sum + (v.chainInfo ? parseFloat(v.chainInfo.votingPower) : 0), 0).toLocaleString()}
              </p>
              <p className="text-xs text-muted-foreground">Total Voting Power</p>
            </div>
          </div>
        </Card>
      </div>

      {/* Search */}
      <div className="relative">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
        <Input
          placeholder="Search validators..."
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          className="pl-10"
        />
      </div>

      {/* Validators Table */}
      <Card className="glass-card">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Validator</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Mode</TableHead>
              <TableHead>Voting Power</TableHead>
              <TableHead>Commission</TableHead>
              <TableHead>Jailed</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filteredValidators.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} className="text-center py-8 text-muted-foreground">
                  {searchTerm ? 'No validators found matching your search' : 'No validators found'}
                </TableCell>
              </TableRow>
            ) : (
              filteredValidators.map((validator) => (
                <TableRow key={validator.setupRequest.id}>
                  <TableCell className="font-medium">
                    <div className="flex items-center gap-2">
                      <Users className="h-4 w-4 text-muted-foreground" />
                      {validator.setupRequest.validatorName}
                    </div>
                  </TableCell>
                  <TableCell>
                    {getStatusBadge(validator.chainInfo?.status || validator.setupRequest.status)}
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline">
                      {validator.setupRequest.runMode === 'cloud' ? 'Cloud' : 'Local'}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    {validator.chainInfo?.votingPower
                      ? parseFloat(validator.chainInfo.votingPower).toLocaleString()
                      : 'N/A'
                    }
                  </TableCell>
                  <TableCell>
                    {validator.chainInfo?.commission?.commissionRates?.rate
                      ? `${(parseFloat(validator.chainInfo.commission.commissionRates.rate) * 100).toFixed(2)}%`
                      : 'N/A'
                    }
                  </TableCell>
                  <TableCell>
                    {validator.chainInfo?.jailed ? (
                      <Badge variant="destructive">Yes</Badge>
                    ) : validator.chainInfo ? (
                      <Badge variant="outline">No</Badge>
                    ) : (
                      'N/A'
                    )}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => {
                        // Navigate to validator details
                        window.location.href = `/dashboard?validator=${validator.setupRequest.id}`;
                      }}
                    >
                      View
                      <ExternalLink className="ml-2 h-3 w-3" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </Card>

      {/* Empty State for New Users */}
      {validators.length === 0 && !loading && (
        <Card className="glass-card p-12 text-center">
          <div className="max-w-md mx-auto space-y-4">
            <div className="mx-auto w-16 h-16 rounded-full bg-primary/10 flex items-center justify-center">
              <Users className="h-8 w-8 text-primary" />
            </div>
            <h3 className="text-xl font-bold">No Validators Yet</h3>
            <p className="text-muted-foreground">
              You haven't created any validators yet. Get started by setting up your first validator.
            </p>
            <Button onClick={() => window.location.href = '/wizard'} size="lg" className="glow-primary">
              Create Validator
            </Button>
          </div>
        </Card>
      )}
    </div>
  );
};

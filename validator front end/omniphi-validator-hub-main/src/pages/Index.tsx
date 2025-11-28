import { WalletConnect } from '@/components/WalletConnect';
import { Button } from '@/components/ui/button';
import { Sparkles, Shield, Zap } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { useValidatorStore } from '@/store/validatorStore';

const Index = () => {
  const navigate = useNavigate();
  const { isWalletConnected } = useValidatorStore();

  const handleGetStarted = () => {
    if (isWalletConnected) {
      navigate('/wizard');
    }
  };

  return (
    <div className="min-h-screen bg-gradient-to-b from-background via-background to-background/80">
      {/* Header */}
      <header className="border-b border-border/50 backdrop-blur-sm sticky top-0 z-50 bg-background/80">
        <div className="container mx-auto px-4 py-4 flex items-center justify-between">
          <button 
            onClick={() => navigate('/')}
            className="flex items-center gap-2 group transition-all hover:opacity-80 cursor-pointer"
          >
            <div className="h-10 w-10 rounded-lg bg-gradient-to-br from-primary to-accent flex items-center justify-center group-hover:scale-105 transition-transform">
              <Shield className="h-6 w-6 text-primary-foreground" />
            </div>
            <span className="text-xl font-bold gradient-text">Omniphi Validators</span>
          </button>
          <WalletConnect />
        </div>
      </header>

      {/* Main Content */}
      <main className="container mx-auto px-4 py-12">
        <div className="max-w-5xl mx-auto space-y-12 animate-fade-in">
          {/* Hero */}
          <div className="text-center space-y-6">
            <div className="inline-block">
              <div className="px-4 py-2 rounded-full bg-primary/10 border border-primary/20 text-sm font-medium text-primary">
                One-Click Validator Setup
              </div>
            </div>
            
            <h1 className="text-5xl md:text-6xl font-bold leading-tight">
              Launch Your Validator
              <br />
              <span className="gradient-text">In Minutes</span>
            </h1>
            
            <p className="text-xl text-muted-foreground max-w-2xl mx-auto">
              Join the Omniphi network as a validator. Choose between managed cloud hosting 
              or self-hosted infrastructure with our streamlined setup process.
            </p>

            {!isWalletConnected && (
              <div className="pt-4">
                <WalletConnect />
              </div>
            )}

            {isWalletConnected && (
              <div className="pt-4">
                <Button size="lg" onClick={handleGetStarted} className="glow-primary">
                  Get Started
                  <Sparkles className="ml-2 h-5 w-5" />
                </Button>
              </div>
            )}
          </div>

          {/* Features */}
          <div className="grid md:grid-cols-3 gap-6 pt-8">
            <div className="glass-card p-6 space-y-3 animate-slide-up" style={{ animationDelay: '0.1s' }}>
              <div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center">
                <Zap className="h-6 w-6 text-primary" />
              </div>
              <h3 className="text-lg font-bold">Quick Setup</h3>
              <p className="text-sm text-muted-foreground">
                Get your validator running in minutes with our guided wizard and automated infrastructure
              </p>
            </div>

            <div className="glass-card p-6 space-y-3 animate-slide-up" style={{ animationDelay: '0.2s' }}>
              <div className="h-12 w-12 rounded-lg bg-accent/10 flex items-center justify-center">
                <Shield className="h-6 w-6 text-accent" />
              </div>
              <h3 className="text-lg font-bold">Enterprise Security</h3>
              <p className="text-sm text-muted-foreground">
                Industry-leading security practices with automated monitoring and instant alerts
              </p>
            </div>

            <div className="glass-card p-6 space-y-3 animate-slide-up" style={{ animationDelay: '0.3s' }}>
              <div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center">
                <Sparkles className="h-6 w-6 text-primary" />
              </div>
              <h3 className="text-lg font-bold">Full Control</h3>
              <p className="text-sm text-muted-foreground">
                Choose between cloud-managed or self-hosted solutions based on your needs
              </p>
            </div>
          </div>
        </div>
      </main>

      {/* Footer */}
      <footer className="border-t border-border/50 mt-24">
        <div className="container mx-auto px-4 py-6">
          <div className="flex items-center justify-between text-sm text-muted-foreground">
            <p>&copy; 2024 Omniphi. All rights reserved.</p>
            <div className="flex gap-6">
              <a href="#" className="hover:text-primary transition-colors">Documentation</a>
              <a href="#" className="hover:text-primary transition-colors">Support</a>
              <a href="#" className="hover:text-primary transition-colors">Terms</a>
            </div>
          </div>
        </div>
      </footer>
    </div>
  );
};

export default Index;

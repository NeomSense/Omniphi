/**
 * Become a Provider Page - Application Form
 */

import { useState } from 'react';
import { Link } from 'react-router-dom';
import {
  CheckCircle,
  Building,
  Globe,
  MapPin,
  DollarSign,
  Server,
  FileText,
  Mail,
  User,
  ArrowLeft,
  Send,
} from 'lucide-react';
import { api } from '../services/api';
import type { InfrastructureType } from '../types';

const regions = [
  'US-East',
  'US-West',
  'EU-West',
  'EU-Central',
  'Asia-Pacific',
  'Asia-Singapore',
  'South America',
  'Africa',
];

export function BecomeProviderPage() {
  const [submitted, setSubmitted] = useState(false);
  const [loading, setLoading] = useState(false);

  const [form, setForm] = useState({
    provider_name: '',
    website: '',
    contact_email: '',
    contact_name: '',
    regions: [] as string[],
    infrastructure_type: 'cloud' as InfrastructureType,
    pricing_model: 'monthly',
    price_per_month: 0,
    api_endpoint: '',
    documentation_url: '',
    capacity: 100,
    proof_of_capacity: '',
  });

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);

    const result = await api.applications.submit(form);

    if (result.success) {
      setSubmitted(true);
    }

    setLoading(false);
  };

  const updateForm = <K extends keyof typeof form>(key: K, value: (typeof form)[K]) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  };

  const toggleRegion = (region: string) => {
    setForm((prev) => ({
      ...prev,
      regions: prev.regions.includes(region)
        ? prev.regions.filter((r) => r !== region)
        : [...prev.regions, region],
    }));
  };

  if (submitted) {
    return (
      <div className="max-w-2xl mx-auto px-4 py-16 text-center">
        <div className="w-20 h-20 bg-green-900/30 rounded-full flex items-center justify-center mx-auto mb-6">
          <CheckCircle className="w-10 h-10 text-green-400" />
        </div>
        <h1 className="text-3xl font-bold text-white mb-4">Application Submitted!</h1>
        <p className="text-lg text-dark-400 mb-8">
          Thank you for your interest in becoming an Omniphi hosting provider.
          Our team will review your application and get back to you within 3-5 business days.
        </p>
        <Link to="/" className="btn btn-primary">
          Back to Marketplace
        </Link>
      </div>
    );
  }

  return (
    <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      {/* Back Button */}
      <Link
        to="/"
        className="inline-flex items-center text-dark-400 hover:text-white mb-6"
      >
        <ArrowLeft className="w-4 h-4 mr-2" />
        Back to Marketplace
      </Link>

      {/* Header */}
      <div className="text-center mb-12">
        <h1 className="text-3xl font-bold text-white mb-4">Become a Hosting Provider</h1>
        <p className="text-lg text-dark-400 max-w-2xl mx-auto">
          Join the Omniphi validator marketplace and offer your infrastructure
          to validators worldwide. Earn revenue while supporting network decentralization.
        </p>
      </div>

      {/* Benefits */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-12">
        <div className="card text-center">
          <div className="w-12 h-12 bg-omniphi-900/30 rounded-lg flex items-center justify-center mx-auto mb-4">
            <DollarSign className="w-6 h-6 text-omniphi-400" />
          </div>
          <h3 className="font-semibold text-white mb-2">Earn Revenue</h3>
          <p className="text-sm text-dark-400">
            Set your own pricing and earn monthly recurring revenue from validators
          </p>
        </div>
        <div className="card text-center">
          <div className="w-12 h-12 bg-blue-900/30 rounded-lg flex items-center justify-center mx-auto mb-4">
            <Globe className="w-6 h-6 text-blue-400" />
          </div>
          <h3 className="font-semibold text-white mb-2">Global Reach</h3>
          <p className="text-sm text-dark-400">
            Access validators from around the world looking for reliable hosting
          </p>
        </div>
        <div className="card text-center">
          <div className="w-12 h-12 bg-green-900/30 rounded-lg flex items-center justify-center mx-auto mb-4">
            <Server className="w-6 h-6 text-green-400" />
          </div>
          <h3 className="font-semibold text-white mb-2">Easy Integration</h3>
          <p className="text-sm text-dark-400">
            Simple API integration with our orchestrator for automated provisioning
          </p>
        </div>
      </div>

      {/* Application Form */}
      <form onSubmit={handleSubmit} className="card">
        <h2 className="text-xl font-bold text-white mb-6">Provider Application</h2>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* Company Info */}
          <div>
            <label className="label">
              <Building className="w-4 h-4 inline mr-1" />
              Provider Name *
            </label>
            <input
              type="text"
              value={form.provider_name}
              onChange={(e) => updateForm('provider_name', e.target.value)}
              className="input"
              placeholder="Your Company Name"
              required
            />
          </div>

          <div>
            <label className="label">
              <Globe className="w-4 h-4 inline mr-1" />
              Website *
            </label>
            <input
              type="url"
              value={form.website}
              onChange={(e) => updateForm('website', e.target.value)}
              className="input"
              placeholder="https://yourcompany.com"
              required
            />
          </div>

          {/* Contact Info */}
          <div>
            <label className="label">
              <User className="w-4 h-4 inline mr-1" />
              Contact Name *
            </label>
            <input
              type="text"
              value={form.contact_name}
              onChange={(e) => updateForm('contact_name', e.target.value)}
              className="input"
              placeholder="John Doe"
              required
            />
          </div>

          <div>
            <label className="label">
              <Mail className="w-4 h-4 inline mr-1" />
              Contact Email *
            </label>
            <input
              type="email"
              value={form.contact_email}
              onChange={(e) => updateForm('contact_email', e.target.value)}
              className="input"
              placeholder="contact@yourcompany.com"
              required
            />
          </div>

          {/* Infrastructure */}
          <div>
            <label className="label">
              <Server className="w-4 h-4 inline mr-1" />
              Infrastructure Type *
            </label>
            <select
              value={form.infrastructure_type}
              onChange={(e) => updateForm('infrastructure_type', e.target.value as InfrastructureType)}
              className="select"
            >
              <option value="cloud">Cloud (AWS, GCP, etc.)</option>
              <option value="bare_metal">Bare Metal</option>
              <option value="decentralized">Decentralized (Akash, etc.)</option>
            </select>
          </div>

          <div>
            <label className="label">
              <Server className="w-4 h-4 inline mr-1" />
              Validator Capacity *
            </label>
            <input
              type="number"
              value={form.capacity}
              onChange={(e) => updateForm('capacity', parseInt(e.target.value) || 0)}
              className="input"
              min={10}
              required
            />
            <p className="text-xs text-dark-500 mt-1">
              Maximum number of validators you can host
            </p>
          </div>

          {/* Pricing */}
          <div>
            <label className="label">
              <DollarSign className="w-4 h-4 inline mr-1" />
              Price per Month (USD) *
            </label>
            <input
              type="number"
              value={form.price_per_month}
              onChange={(e) => updateForm('price_per_month', parseInt(e.target.value) || 0)}
              className="input"
              min={0}
              required
            />
          </div>

          <div>
            <label className="label">Pricing Model</label>
            <select
              value={form.pricing_model}
              onChange={(e) => updateForm('pricing_model', e.target.value)}
              className="select"
            >
              <option value="monthly">Monthly Subscription</option>
              <option value="epoch">Per Epoch</option>
              <option value="usage">Usage-Based</option>
            </select>
          </div>

          {/* API Integration */}
          <div className="md:col-span-2">
            <label className="label">
              API Endpoint for Orchestrator Integration
            </label>
            <input
              type="url"
              value={form.api_endpoint}
              onChange={(e) => updateForm('api_endpoint', e.target.value)}
              className="input"
              placeholder="https://api.yourcompany.com/v1"
            />
            <p className="text-xs text-dark-500 mt-1">
              Required for automated provisioning. Leave blank if not ready yet.
            </p>
          </div>

          <div className="md:col-span-2">
            <label className="label">
              <FileText className="w-4 h-4 inline mr-1" />
              Documentation URL
            </label>
            <input
              type="url"
              value={form.documentation_url}
              onChange={(e) => updateForm('documentation_url', e.target.value)}
              className="input"
              placeholder="https://docs.yourcompany.com"
            />
          </div>
        </div>

        {/* Regions */}
        <div className="mt-6">
          <label className="label">
            <MapPin className="w-4 h-4 inline mr-1" />
            Supported Regions *
          </label>
          <div className="flex flex-wrap gap-2">
            {regions.map((region) => (
              <button
                key={region}
                type="button"
                onClick={() => toggleRegion(region)}
                className={`filter-chip ${
                  form.regions.includes(region) ? 'filter-chip-active' : 'filter-chip-inactive'
                }`}
              >
                {region}
              </button>
            ))}
          </div>
        </div>

        {/* Proof of Capacity */}
        <div className="mt-6">
          <label className="label">
            Proof of Capacity (Optional)
          </label>
          <textarea
            value={form.proof_of_capacity}
            onChange={(e) => updateForm('proof_of_capacity', e.target.value)}
            className="input h-24 resize-none"
            placeholder="Describe your infrastructure, certifications, experience with other chains, etc."
          />
        </div>

        {/* Submit */}
        <div className="mt-8 flex items-center justify-between">
          <p className="text-sm text-dark-500">
            By submitting, you agree to our Provider Terms of Service
          </p>
          <button
            type="submit"
            disabled={loading || !form.provider_name || !form.contact_email || form.regions.length === 0}
            className="btn btn-primary btn-lg"
          >
            {loading ? (
              <>
                <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                Submitting...
              </>
            ) : (
              <>
                <Send className="w-4 h-4 mr-2" />
                Submit Application
              </>
            )}
          </button>
        </div>
      </form>
    </div>
  );
}

function RefreshCw({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
    </svg>
  );
}

export default BecomeProviderPage;
